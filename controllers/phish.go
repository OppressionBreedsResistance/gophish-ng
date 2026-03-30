package controllers

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/gophish/gophish/config"
	ctx "github.com/gophish/gophish/context"
	"github.com/gophish/gophish/controllers/api"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/gophish/gophish/util"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/jordan-wright/unindexed"
)

func customError(w http.ResponseWriter, error string, code int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	fmt.Fprintln(w, error)
}

func customNotFound(w http.ResponseWriter, r *http.Request) {
	tmpl404, err := template.ParseFiles("templates/404.html")
	if err != nil {
		log.Fatal(err)
	}
	var b bytes.Buffer
	err = tmpl404.Execute(&b, "")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	customError(w, b.String(), http.StatusNotFound)
}

// ErrInvalidRequest is thrown when a request with an invalid structure is
// received
var ErrInvalidRequest = errors.New("Invalid request")

// ErrCampaignComplete is thrown when an event is received for a campaign that
// has already been marked as complete.
var ErrCampaignComplete = errors.New("Event received on completed campaign")

// TransparencyResponse is the JSON response provided when a third-party
// makes a request to the transparency handler.
type TransparencyResponse struct {
	Server         string    `json:"server"`
	ContactAddress string    `json:"contact_address"`
	SendDate       time.Time `json:"send_date"`
}

// TransparencySuffix (when appended to a valid result ID), will cause Gophish
// to return a transparency response.
const TransparencySuffix = "+"

// PhishingServerOption is a functional option that is used to configure the
// the phishing server
type PhishingServerOption func(*PhishingServer)

// PhishingServer is an HTTP server that implements the campaign event
// handlers, such as email open tracking, click tracking, and more.
type PhishingServer struct {
	server         *http.Server
	config         config.PhishServer
	contactAddress string
	turnstile      config.TurnstileConfig
}

// turnstileCookieName is the name of the cookie set after a successful Turnstile challenge.
const turnstileCookieName = "ts_v"

// turnstileCookieTTL is how long a verified Turnstile session cookie is valid.
const turnstileCookieTTL = 3600 // 1 hour

// turnstileChallengeTmpl is the HTML page shown to visitors before they can access a landing page.
var turnstileChallengeTmpl = template.Must(template.New("ts").Parse(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Cloudflare</title>
<script src="https://challenges.cloudflare.com/turnstile/v0/api.js" async defer></script>
<style>
body{margin:0;display:flex;justify-content:center;align-items:center;min-height:100vh;background:#f6f6ef}
</style>
</head>
<body>
<form id="tsf" action="/ts-verify" method="POST">
  <input type="hidden" name="{{.Param}}" value="{{.RId}}">
  <div class="cf-turnstile" data-sitekey="{{.SiteKey}}" data-callback="onSuccess"></div>
</form>
<script>function onSuccess(){document.getElementById("tsf").submit()}</script>
</body>
</html>`))

// turnstileVerifyResponse maps the fields we care about from Cloudflare's siteverify API.
type turnstileVerifyResponse struct {
	Success bool `json:"success"`
}

// NewPhishingServer returns a new instance of the phishing server with
// provided options applied.
func NewPhishingServer(config config.PhishServer, options ...PhishingServerOption) *PhishingServer {
	defaultServer := &http.Server{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		Addr:         config.ListenURL,
	}
	ps := &PhishingServer{
		server: defaultServer,
		config: config,
	}
	for _, opt := range options {
		opt(ps)
	}
	ps.registerRoutes()
	return ps
}

// WithContactAddress sets the contact address used by the transparency
// handlers
func WithContactAddress(addr string) PhishingServerOption {
	return func(ps *PhishingServer) {
		ps.contactAddress = addr
	}
}

// WithTurnstile configures Cloudflare Turnstile bot protection on the phishing server.
func WithTurnstile(tc config.TurnstileConfig) PhishingServerOption {
	return func(ps *PhishingServer) {
		ps.turnstile = tc
	}
}

// turnstileEnabled returns true when both Turnstile keys are configured.
func (ps *PhishingServer) turnstileEnabled() bool {
	return ps.turnstile.SiteKey != "" && ps.turnstile.SecretKey != ""
}

// turnstileSign returns an HMAC-SHA256 signature over the given message using the secret key.
func (ps *PhishingServer) turnstileSign(message string) string {
	mac := hmac.New(sha256.New, []byte(ps.turnstile.SecretKey))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// isTurnstileVerified checks whether the request carries a valid Turnstile session cookie for the given RId.
func (ps *PhishingServer) isTurnstileVerified(r *http.Request, rid string) bool {
	cookie, err := r.Cookie(turnstileCookieName)
	if err != nil {
		return false
	}
	parts := strings.SplitN(cookie.Value, "|", 3)
	if len(parts) != 3 {
		return false
	}
	cookieRid, tsStr, sig := parts[0], parts[1], parts[2]
	if cookieRid != rid {
		return false
	}
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix()-ts > turnstileCookieTTL {
		return false
	}
	expected := ps.turnstileSign(cookieRid + "|" + tsStr)
	return hmac.Equal([]byte(sig), []byte(expected))
}

// setTurnstileCookie writes a signed session cookie for the given RId.
func (ps *PhishingServer) setTurnstileCookie(w http.ResponseWriter, rid string) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	payload := rid + "|" + ts
	sig := ps.turnstileSign(payload)
	http.SetCookie(w, &http.Cookie{
		Name:     turnstileCookieName,
		Value:    payload + "|" + sig,
		MaxAge:   turnstileCookieTTL,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
}

// verifyTurnstileToken calls the Cloudflare siteverify API and returns true on success.
func (ps *PhishingServer) verifyTurnstileToken(token, remoteIP string) bool {
	resp, err := http.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify",
		url.Values{
			"secret":   {ps.turnstile.SecretKey},
			"response": {token},
			"remoteip": {remoteIP},
		})
	if err != nil {
		log.Warnf("turnstile: siteverify request failed: %v", err)
		return false
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}
	var result turnstileVerifyResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return false
	}
	return result.Success
}

// serveTurnstileChallenge renders the Turnstile challenge page.
func (ps *PhishingServer) serveTurnstileChallenge(w http.ResponseWriter, rid string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := turnstileChallengeTmpl.Execute(w, struct {
		SiteKey string
		RId     string
		Param   string
	}{
		SiteKey: ps.turnstile.SiteKey,
		RId:     rid,
		Param:   models.RecipientParameter,
	})
	if err != nil {
		log.Error(err)
	}
}

// TurnstileVerifyHandler handles the POST from the Turnstile challenge page,
// verifies the token with Cloudflare, and on success sets a session cookie
// before redirecting the visitor back to the phishing page.
func (ps *PhishingServer) TurnstileVerifyHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	rid := r.FormValue(models.RecipientParameter)
	token := r.FormValue("cf-turnstile-response")
	if rid == "" || token == "" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	if !ps.verifyTurnstileToken(token, ip) {
		log.Warnf("turnstile: challenge failed for rid %s", rid)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	ps.setTurnstileCookie(w, rid)
	returnURL := fmt.Sprintf("/?%s=%s", models.RecipientParameter, url.QueryEscape(rid))
	http.Redirect(w, r, returnURL, http.StatusFound)
}

// Start launches the phishing server, listening on the configured address.
func (ps *PhishingServer) Start() {
	if ps.config.UseTLS {
		// Only support TLS 1.2 and above - ref #1691, #1689
		ps.server.TLSConfig = defaultTLSConfig
		err := util.CheckAndCreateSSL(ps.config.CertPath, ps.config.KeyPath)
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("Starting phishing server at https://%s", ps.config.ListenURL)
		log.Fatal(ps.server.ListenAndServeTLS(ps.config.CertPath, ps.config.KeyPath))
	}
	// If TLS isn't configured, just listen on HTTP
	log.Infof("Starting phishing server at http://%s", ps.config.ListenURL)
	log.Fatal(ps.server.ListenAndServe())
}

// Shutdown attempts to gracefully shutdown the server.
func (ps *PhishingServer) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	return ps.server.Shutdown(ctx)
}

// extractRIdFromPath pulls the RId out of a hosted-attachment path:
// /static/attachments/<campaignId>/<RId>/<filename>
func extractRIdFromPath(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	// ["static", "attachments", "<campaignId>", "<RId>", "<filename>"]
	if len(parts) >= 5 && parts[0] == "static" && parts[1] == "attachments" {
		return parts[3]
	}
	return ""
}

// turnstileMiddleware wraps the entire phishing router and enforces the
// Cloudflare Turnstile challenge for every request except the exempt paths.
func (ps *PhishingServer) turnstileMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !ps.turnstileEnabled() {
			next.ServeHTTP(w, r)
			return
		}
		path := r.URL.Path
		// Exempt: verification endpoint, email-open pixels, attachment tracking
		// (attachment payloads are scripts/macros with no browser session)
		// and robots.
		if path == "/ts-verify" ||
			path == "/track" || strings.HasSuffix(path, "/track") ||
			path == "/attachment" || strings.HasSuffix(path, "/attachment") ||
			path == "/robots.txt" {
			next.ServeHTTP(w, r)
			return
		}
		// Extract RId from query param (most routes) or from path (hosted attachments)
		rid := r.URL.Query().Get(models.RecipientParameter)
		if rid == "" {
			rid = extractRIdFromPath(path)
		}
		if rid == "" || !ps.isTurnstileVerified(r, rid) {
			if rid == "" {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			ps.serveTurnstileChallenge(w, rid)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CreatePhishingRouter creates the router that handles phishing connections.
func (ps *PhishingServer) registerRoutes() {
	router := mux.NewRouter()
	fileServer := http.FileServer(unindexed.Dir("./static/endpoint/"))
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fileServer))
	router.HandleFunc("/ts-verify", ps.TurnstileVerifyHandler).Methods("POST")
	router.HandleFunc("/track", ps.TrackHandler)
	router.HandleFunc("/robots.txt", ps.RobotsHandler)
	router.HandleFunc("/{path:.*}/track", ps.TrackHandler)
	router.HandleFunc("/{path:.*}/report", ps.ReportHandler)
	router.HandleFunc("/report", ps.ReportHandler)
	router.HandleFunc("/{path:.*}/attachment", ps.AttachmentHandler)
	router.HandleFunc("/attachment", ps.AttachmentHandler)
	router.HandleFunc("/{path:.*}", ps.PhishHandler)

	// Setup GZIP compression
	gzipWrapper, _ := gziphandler.NewGzipLevelHandler(gzip.BestCompression)
	phishHandler := gzipWrapper(router)

	// Respect X-Forwarded-For and X-Real-IP headers in case we're behind a
	// reverse proxy.
	phishHandler = handlers.ProxyHeaders(phishHandler)

	// Cloudflare Turnstile — wraps the entire router (no-op when keys are empty)
	phishHandler = ps.turnstileMiddleware(phishHandler)

	// Setup logging
	phishHandler = handlers.CombinedLoggingHandler(log.Writer(), phishHandler)
	ps.server.Handler = phishHandler
}

// TrackHandler tracks emails as they are opened, updating the status for the given Result
func (ps *PhishingServer) TrackHandler(w http.ResponseWriter, r *http.Request) {
	r, err := setupContext(r)
	if err != nil {
		// Log the error if it wasn't something we can safely ignore
		if err != ErrInvalidRequest && err != ErrCampaignComplete {
			log.Error(err)
		}
		customNotFound(w, r)
		return
	}
	// Check for a preview
	if _, ok := ctx.Get(r, "result").(models.EmailRequest); ok {
		http.ServeFile(w, r, "static/images/pixel.png")
		return
	}
	rs := ctx.Get(r, "result").(models.Result)
	rid := ctx.Get(r, "rid").(string)
	d := ctx.Get(r, "details").(models.EventDetails)

	// Check for a transparency request
	if strings.HasSuffix(rid, TransparencySuffix) {
		ps.TransparencyHandler(w, r)
		return
	}

	err = rs.HandleEmailOpened(d)
	if err != nil {
		log.Error(err)
	}
	http.ServeFile(w, r, "static/images/pixel.png")
}

// ReportHandler tracks emails as they are reported, updating the status for the given Result
func (ps *PhishingServer) ReportHandler(w http.ResponseWriter, r *http.Request) {
	r, err := setupContext(r)
	w.Header().Set("Access-Control-Allow-Origin", "*") // To allow Chrome extensions (or other pages) to report a campaign without violating CORS
	if err != nil {
		// Log the error if it wasn't something we can safely ignore
		if err != ErrInvalidRequest && err != ErrCampaignComplete {
			log.Error(err)
		}
		customNotFound(w, r)
		return
	}
	// Check for a preview
	if _, ok := ctx.Get(r, "result").(models.EmailRequest); ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	rs := ctx.Get(r, "result").(models.Result)
	rid := ctx.Get(r, "rid").(string)
	d := ctx.Get(r, "details").(models.EventDetails)

	// Check for a transparency request
	if strings.HasSuffix(rid, TransparencySuffix) {
		ps.TransparencyHandler(w, r)
		return
	}

	err = rs.HandleEmailReport(d)
	if err != nil {
		log.Error(err)
	}
	w.WriteHeader(http.StatusNoContent)
}

// AttachmentHandler tracks when a recipient executes the tracked attachment payload,
// updating the status for the given Result.
func (ps *PhishingServer) AttachmentHandler(w http.ResponseWriter, r *http.Request) {
	r, err := setupContext(r)
	if err != nil {
		if err != ErrInvalidRequest && err != ErrCampaignComplete {
			log.Error(err)
		}
		customNotFound(w, r)
		return
	}
	if _, ok := ctx.Get(r, "result").(models.EmailRequest); ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	rs := ctx.Get(r, "result").(models.Result)
	rid := ctx.Get(r, "rid").(string)
	d := ctx.Get(r, "details").(models.EventDetails)
	if strings.HasSuffix(rid, TransparencySuffix) {
		ps.TransparencyHandler(w, r)
		return
	}
	err = rs.HandleAttachmentOpened(d)
	if err != nil {
		log.Error(err)
	}
	w.WriteHeader(http.StatusNoContent)
}

// PhishHandler handles incoming client connections and registers the associated actions performed
// (such as clicked link, etc.)
func (ps *PhishingServer) PhishHandler(w http.ResponseWriter, r *http.Request) {
	r, err := setupContext(r)
	if err != nil {
		// Log the error if it wasn't something we can safely ignore
		if err != ErrInvalidRequest && err != ErrCampaignComplete {
			log.Error(err)
		}
		customNotFound(w, r)
		return
	}
	w.Header().Set("X-Server", config.ServerName) // Useful for checking if this is a GoPhish server (e.g. for campaign reporting plugins)
	var ptx models.PhishingTemplateContext
	// Check for a preview
	if preview, ok := ctx.Get(r, "result").(models.EmailRequest); ok {
		ptx, err = models.NewPhishingTemplateContext(&preview, preview.BaseRecipient, preview.RId)
		if err != nil {
			log.Error(err)
			customNotFound(w, r)
			return
		}
		p, err := models.GetPage(preview.PageId, preview.UserId)
		if err != nil {
			log.Error(err)
			customNotFound(w, r)
			return
		}
		renderPhishResponse(w, r, ptx, p)
		return
	}
	rs := ctx.Get(r, "result").(models.Result)
	rid := ctx.Get(r, "rid").(string)
	c := ctx.Get(r, "campaign").(models.Campaign)
	d := ctx.Get(r, "details").(models.EventDetails)

	// Check for a transparency request
	if strings.HasSuffix(rid, TransparencySuffix) {
		ps.TransparencyHandler(w, r)
		return
	}

	p, err := models.GetPage(c.PageId, c.UserId)
	if err != nil {
		log.Error(err)
		customNotFound(w, r)
		return
	}
	switch {
	case r.Method == "GET":
		err = rs.HandleClickedLink(d)
		if err != nil {
			log.Error(err)
		}
		if c.HostAttachment && len(c.Template.Attachments) > 0 {
			safeFilename := filepath.Base(c.Template.Attachments[0].Name)
			attachmentURL := fmt.Sprintf("/static/attachments/%d/%s/%s", c.Id, rs.RId, url.PathEscape(safeFilename))
			http.Redirect(w, r, attachmentURL, http.StatusFound)
			return
		}
	case r.Method == "POST":
		err = rs.HandleFormSubmit(d)
		if err != nil {
			log.Error(err)
		}
	}
	ptx, err = models.NewPhishingTemplateContext(&c, rs.BaseRecipient, rs.RId)
	if err != nil {
		log.Error(err)
		customNotFound(w, r)
	}
	renderPhishResponse(w, r, ptx, p)
}

// renderPhishResponse handles rendering the correct response to the phishing
// connection. This usually involves writing out the page HTML or redirecting
// the user to the correct URL.
func renderPhishResponse(w http.ResponseWriter, r *http.Request, ptx models.PhishingTemplateContext, p models.Page) {
	// If the request was a form submit and a redirect URL was specified, we
	// should send the user to that URL
	if r.Method == "POST" {
		if p.RedirectURL != "" {
			redirectURL, err := models.ExecuteTemplate(p.RedirectURL, ptx)
			if err != nil {
				log.Error(err)
				customNotFound(w, r)
				return
			}
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return
		}
	}
	// Otherwise, we just need to write out the templated HTML
	html, err := models.ExecuteTemplate(p.HTML, ptx)
	if err != nil {
		log.Error(err)
		customNotFound(w, r)
		return
	}
	w.Write([]byte(html))
}

// RobotsHandler prevents search engines, etc. from indexing phishing materials
func (ps *PhishingServer) RobotsHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "User-agent: *\nDisallow: /")
}

// TransparencyHandler returns a TransparencyResponse for the provided result
// and campaign.
func (ps *PhishingServer) TransparencyHandler(w http.ResponseWriter, r *http.Request) {
	rs := ctx.Get(r, "result").(models.Result)
	tr := &TransparencyResponse{
		Server:         config.ServerName,
		SendDate:       rs.SendDate,
		ContactAddress: ps.contactAddress,
	}
	api.JSONResponse(w, tr, http.StatusOK)
}

// setupContext handles some of the administrative work around receiving a new
// request, such as checking the result ID, the campaign, etc.
func setupContext(r *http.Request) (*http.Request, error) {
	err := r.ParseForm()
	if err != nil {
		log.Error(err)
		return r, err
	}
	rid := r.Form.Get(models.RecipientParameter)
	if rid == "" {
		return r, ErrInvalidRequest
	}
	// Since we want to support the common case of adding a "+" to indicate a
	// transparency request, we need to take care to handle the case where the
	// request ends with a space, since a "+" is technically reserved for use
	// as a URL encoding of a space.
	if strings.HasSuffix(rid, " ") {
		// We'll trim off the space
		rid = strings.TrimRight(rid, " ")
		// Then we'll add the transparency suffix
		rid = fmt.Sprintf("%s%s", rid, TransparencySuffix)
	}
	// Finally, if this is a transparency request, we'll need to verify that
	// a valid rid has been provided, so we'll look up the result with a
	// trimmed parameter.
	id := strings.TrimSuffix(rid, TransparencySuffix)
	// Check to see if this is a preview or a real result
	if strings.HasPrefix(id, models.PreviewPrefix) {
		rs, err := models.GetEmailRequestByResultId(id)
		if err != nil {
			return r, err
		}
		r = ctx.Set(r, "result", rs)
		return r, nil
	}
	rs, err := models.GetResult(id)
	if err != nil {
		return r, err
	}
	c, err := models.GetCampaign(rs.CampaignId, rs.UserId)
	if err != nil {
		log.Error(err)
		return r, err
	}
	// Don't process events for completed campaigns
	if c.Status == models.CampaignComplete {
		return r, ErrCampaignComplete
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	// Handle post processing such as GeoIP
	err = rs.UpdateGeo(ip)
	if err != nil {
		log.Error(err)
	}
	d := models.EventDetails{
		Payload: r.Form,
		Browser: make(map[string]string),
	}
	d.Browser["address"] = ip
	d.Browser["user-agent"] = r.Header.Get("User-Agent")

	r = ctx.Set(r, "rid", rid)
	r = ctx.Set(r, "result", rs)
	r = ctx.Set(r, "campaign", c)
	r = ctx.Set(r, "details", d)
	return r, nil
}
