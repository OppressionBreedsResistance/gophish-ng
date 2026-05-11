#!/bin/bash
# =============================================================================
# Gophish-NG VPS Setup Script
# Installs and configures Gophish-NG with nginx reverse proxy + acme.sh TLS
# Run as root (or with sudo): sudo bash setup_vps.sh
# =============================================================================

set -e

# --- Colors ------------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

info()    { echo -e "${BLUE}[INFO]${NC}  $*"; }
success() { echo -e "${GREEN}[OK]${NC}    $*"; }
warn()    { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error()   { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }
step()    { echo -e "\n${CYAN}${BOLD}==> $*${NC}"; }
ask()     { echo -e "${YELLOW}[?]${NC}    $*"; }

# --- Paths & constants -------------------------------------------------------
GOPHISH_DIR="/opt/gophish-ng"
GOPHISH_USER="gophish"
GOPHISH_REPO="https://github.com/OppressionBreedsResistance/gophish-ng.git"
GOPHISH_BRANCH="master"

ACME_HOME="/root/.acme.sh"
ACME_CERTS_DIR="/etc/ssl/gophish"
WEBROOT="/var/www/acme-challenge"

ADMIN_LISTEN="127.0.0.1:3333"
PHISH_LISTEN="127.0.0.1:5555"

# =============================================================================
# Root check
# =============================================================================
if [[ $EUID -ne 0 ]]; then
    error "Run as root: sudo bash $0"
fi

# =============================================================================
# Banner
# =============================================================================
echo -e "${BOLD}"
cat <<'EOF'
   ____             _     _     _       _   _  ____
  / ___| ___  _ __ | |__ (_)___| |__   | \ | |/ ___|
 | |  _ / _ \| '_ \| '_ \| / __| '_ \  |  \| | |  _
 | |_| | (_) | |_) | | | | \__ \ | | | | |\  | |_| |
  \____|\___/| .__/|_| |_|_|___/_| |_| |_| \_|\____|
             |_|
EOF
echo -e "${NC}${CYAN}  VPS Auto-Setup  —  nginx + acme.sh + Gophish-NG${NC}\n"

# =============================================================================
# Interactive: domain collection
# =============================================================================
step "Domain configuration"

while true; do
    ask "How many phishing domains will you configure? "
    read -r DOMAIN_COUNT
    if [[ "$DOMAIN_COUNT" =~ ^[1-9][0-9]*$ ]]; then
        break
    fi
    warn "Please enter a positive integer."
done

DOMAINS=()
for (( i=1; i<=DOMAIN_COUNT; i++ )); do
    while true; do
        ask "Domain $i (e.g. phish.example.com): "
        read -r domain
        domain="${domain,,}"   # lowercase
        domain="${domain#https://}"
        domain="${domain#http://}"
        domain="${domain%/}"
        if [[ "$domain" =~ ^[a-z0-9]([a-z0-9\-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9\-]{0,61}[a-z0-9])?)+$ ]]; then
            DOMAINS+=("$domain")
            break
        fi
        warn "Invalid domain name, try again."
    done
done

ask "Email address for acme.sh / Let's Encrypt notifications: "
read -r ACME_EMAIL
if [[ -z "$ACME_EMAIL" ]]; then
    error "Email is required for certificate issuance."
fi

echo ""
info "Will configure domains:"
for d in "${DOMAINS[@]}"; do
    echo "    • $d"
done
echo ""
ask "Continue? [y/N] "
read -r confirm
[[ "${confirm,,}" == "y" ]] || error "Aborted by user."

# =============================================================================
# 1. System packages
# =============================================================================
step "Installing system packages"

export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq \
    git curl wget socat nginx golang-go \
    build-essential ca-certificates \
    software-properties-common

success "Packages installed."

# =============================================================================
# 2. Go
# =============================================================================
step "Checking Go installation"

if command -v go &>/dev/null; then
    success "Go $(go version | awk '{print $3}') installed."
else
    error "golang-go installation failed — 'go' not found in PATH."
fi

# =============================================================================
# 3. acme.sh
# =============================================================================
step "Installing acme.sh"

if [[ -f "${ACME_HOME}/acme.sh" ]]; then
    success "acme.sh already installed."
else
    info "Installing acme.sh..."
    curl https://get.acme.sh | sh
    # shellcheck source=/dev/null
    source ~/.bashrc
    success "acme.sh installed."
fi

ACME="${ACME_HOME}/acme.sh"

# =============================================================================
# 4. Start nginx (DNS-01 — no webroot needed)
# =============================================================================
step "Starting nginx"

mkdir -p "${WEBROOT}/.well-known/acme-challenge"
rm -f /etc/nginx/sites-enabled/default
systemctl enable nginx
systemctl restart nginx
success "nginx started."

# =============================================================================
# 5. Issue TLS certificates — DNS-01 manual mode (wildcard)
# =============================================================================
step "Issuing TLS wildcard certificates (DNS-01 manual)"

# acme.sh refuses to run when SUDO_USER is set in the environment
unset SUDO_USER

# acme.sh 3.0+ defaults to ZeroSSL — switch to Let's Encrypt
"${ACME}" --set-default-ca --server letsencrypt --home "${ACME_HOME}"

mkdir -p "${ACME_CERTS_DIR}"

declare -A CERT_PATHS=()
declare -A KEY_PATHS=()

# Step 1: generate DNS challenges — print TXT records directly to terminal
echo ""
echo -e "${BOLD}${YELLOW}================================================================${NC}"
echo -e "${BOLD}${YELLOW}  STEP 1 — Generating DNS challenges (TXT records below)${NC}"
echo -e "${BOLD}${YELLOW}================================================================${NC}"

for domain in "${DOMAINS[@]}"; do
    echo ""
    info "Generating DNS-01 challenge for ${domain} and *.${domain}..."

    CERT_OUT="${ACME_CERTS_DIR}/${domain}"
    mkdir -p "${CERT_OUT}"

    "${ACME}" --issue \
        --home "${ACME_HOME}" \
        -d "${domain}" \
        -d "*.${domain}" \
        --dns \
        --yes-I-know-dns-manual-mode-enough-go-ahead-please \
        --keylength ec-256 \
        || true
done

# Step 2: wait for user to add TXT records
echo ""
echo -e "${BOLD}${YELLOW}================================================================${NC}"
echo -e "${BOLD}${YELLOW}  STEP 2 — Add the TXT records shown above to your DNS${NC}"
echo -e "${BOLD}${YELLOW}================================================================${NC}"
echo ""
warn "Wait at least 60 seconds after adding records for DNS propagation."
ask "Press [Enter] once all TXT records are set and propagated..."
read -r

# Step 3: complete verification and install certs
for domain in "${DOMAINS[@]}"; do
    info "Verifying DNS challenge and issuing certificate for ${domain}..."

    CERT_OUT="${ACME_CERTS_DIR}/${domain}"

    "${ACME}" --renew \
        --home "${ACME_HOME}" \
        -d "${domain}" \
        --ecc \
        || error "Certificate verification failed for ${domain}. Check TXT records and try again."

    "${ACME}" --install-cert \
        --home "${ACME_HOME}" \
        -d "${domain}" \
        --ecc \
        --fullchain-file "${CERT_OUT}/fullchain.cer" \
        --key-file       "${CERT_OUT}/key.pem" \
        --reloadcmd      "systemctl reload nginx"

    CERT_PATHS[$domain]="${CERT_OUT}/fullchain.cer"
    KEY_PATHS[$domain]="${CERT_OUT}/key.pem"
    success "Wildcard certificate installed for ${domain} (covers *.${domain})."
done

# =============================================================================
# 6. nginx — final site configs
# =============================================================================
step "Writing nginx reverse-proxy configs"

for domain in "${DOMAINS[@]}"; do
    SITE_CONF="/etc/nginx/sites-available/${domain}"
    cat > "${SITE_CONF}" <<NGINXEOF
# Gophish-NG phish server — ${domain}
server {
    listen 80;
    server_name ${domain};

    location /.well-known/acme-challenge/ {
        root ${WEBROOT};
    }

    location / {
        return 301 https://\$host\$request_uri;
    }
}

server {
    listen 443 ssl;
    server_name ${domain};

    ssl_certificate     ${CERT_PATHS[$domain]};
    ssl_certificate_key ${KEY_PATHS[$domain]};

    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;

    # Pass all traffic to Gophish phish server
    location / {
        proxy_pass         http://${PHISH_LISTEN};
        proxy_set_header   Host              \$host;
        proxy_set_header   X-Real-IP         \$remote_addr;
        proxy_set_header   X-Forwarded-For   \$proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto \$scheme;
        proxy_read_timeout 90;
    }
}
NGINXEOF

    ln -sf "${SITE_CONF}" /etc/nginx/sites-enabled/"${domain}"
    success "nginx config created for ${domain}."
done

nginx -t && systemctl reload nginx
success "nginx reloaded with final config."

# =============================================================================
# 7. Build Gophish-NG
# =============================================================================
step "Building Gophish-NG from ${GOPHISH_BRANCH}"

if [[ -d "${GOPHISH_DIR}/.git" ]]; then
    info "Repository already exists, pulling latest..."
    git -C "${GOPHISH_DIR}" fetch origin "${GOPHISH_BRANCH}"
    git -C "${GOPHISH_DIR}" reset --hard "origin/${GOPHISH_BRANCH}"
else
    info "Cloning repository..."
    git clone --depth 1 --branch "${GOPHISH_BRANCH}" "${GOPHISH_REPO}" "${GOPHISH_DIR}"
fi

info "Building (this may take a few minutes)..."
cd "${GOPHISH_DIR}"
/usr/local/go/bin/go build -o gophish .
success "Gophish-NG built successfully."

# =============================================================================
# 8. Turnstile configuration
# =============================================================================
step "Cloudflare Turnstile (bot protection)"

TURNSTILE_ENABLED=false
TURNSTILE_SITE_KEY=""
TURNSTILE_SECRET_KEY=""

ask "Czy chcesz zabezpieczyć strony phishingowe Cloudflare Turnstile? [y/N] "
read -r ts_answer
if [[ "${ts_answer,,}" == "y" ]]; then
    while true; do
        ask "Turnstile Site Key: "
        read -r TURNSTILE_SITE_KEY
        [[ -n "$TURNSTILE_SITE_KEY" ]] && break
        warn "Site Key nie może być pusty."
    done
    while true; do
        ask "Turnstile Secret Key: "
        read -r TURNSTILE_SECRET_KEY
        [[ -n "$TURNSTILE_SECRET_KEY" ]] && break
        warn "Secret Key nie może być pusty."
    done
    TURNSTILE_ENABLED=true
    success "Turnstile zostanie skonfigurowany."
else
    info "Turnstile pominięty — można skonfigurować później w config.json."
fi

# =============================================================================
# 9. config.json
# =============================================================================
step "Writing config.json"

if [[ "$TURNSTILE_ENABLED" == true ]]; then
    TURNSTILE_BLOCK='"turnstile": {
        "site_key": "'"${TURNSTILE_SITE_KEY}"'",
        "secret_key": "'"${TURNSTILE_SECRET_KEY}"'"
    }'
else
    TURNSTILE_BLOCK='"turnstile": {
        "site_key": "",
        "secret_key": ""
    }'
fi

cat > "${GOPHISH_DIR}/config.json" <<JSONEOF
{
    "admin_server": {
        "listen_url": "${ADMIN_LISTEN}",
        "use_tls": true,
        "cert_path": "gophish_admin.crt",
        "key_path": "gophish_admin.key",
        "trusted_origins": []
    },
    "phish_server": {
        "listen_url": "${PHISH_LISTEN}",
        "use_tls": false,
        "cert_path": "example.crt",
        "key_path": "example.key"
    },
    "db_name": "sqlite3",
    "db_path": "gophish.db",
    "migrations_prefix": "db/db_",
    "contact_address": "",
    "logging": {
        "filename": "",
        "level": ""
    },
    ${TURNSTILE_BLOCK}
}
JSONEOF

success "config.json written."

# =============================================================================
# 10. Dedicated system user
# =============================================================================
step "Creating system user '${GOPHISH_USER}'"

if id "${GOPHISH_USER}" &>/dev/null; then
    info "User '${GOPHISH_USER}' already exists."
else
    useradd --system --no-create-home --shell /usr/sbin/nologin "${GOPHISH_USER}"
    success "User '${GOPHISH_USER}' created."
fi

chown -R "${GOPHISH_USER}:${GOPHISH_USER}" "${GOPHISH_DIR}"

# =============================================================================
# 11. systemd service
# =============================================================================
step "Creating systemd service"

cat > /etc/systemd/system/gophish.service <<UNITEOF
[Unit]
Description=Gophish-NG Phishing Framework
After=network.target

[Service]
Type=simple
User=${GOPHISH_USER}
WorkingDirectory=${GOPHISH_DIR}
ExecStart=${GOPHISH_DIR}/gophish
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=gophish

# Harden
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
UNITEOF

systemctl daemon-reload
systemctl enable gophish
systemctl restart gophish
success "gophish.service started and enabled."

# =============================================================================
# 12. Certificate renewal notice (DNS-01 manual — no auto-renew)
# =============================================================================
step "Certificate renewal"

warn "Wildcard certificates via DNS-01 manual mode cannot be renewed automatically."
warn "Let's Encrypt certs expire after 90 days."
warn "To renew, repeat the two-step DNS challenge for each domain:"
echo ""
for domain in "${DOMAINS[@]}"; do
    echo -e "  # Step 1 — generate new TXT records:"
    echo -e "  ${ACME} --issue --dns --yes-I-know-dns-manual-mode-enough-go-ahead-please --home ${ACME_HOME} -d ${domain} -d *.${domain} --ecc --force"
    echo -e "  # Step 2 — after updating DNS TXT records:"
    echo -e "  ${ACME} --renew --home ${ACME_HOME} -d ${domain} --ecc"
    echo ""
done
info "Tip: set a calendar reminder ~75 days from today to renew certificates."

# =============================================================================
# Done — print summary
# =============================================================================
echo ""
echo -e "${BOLD}${GREEN}================================================================${NC}"
echo -e "${BOLD}${GREEN}  Setup complete!${NC}"
echo -e "${BOLD}${GREEN}================================================================${NC}"
echo ""
echo -e "${BOLD}Gophish-NG${NC}"
echo -e "  Directory : ${GOPHISH_DIR}"
echo -e "  Service   : systemctl status gophish"
echo -e "  Admin URL : https://${ADMIN_LISTEN}  (access via SSH tunnel)"
echo ""
echo -e "${BOLD}Admin panel access (SSH tunnel):${NC}"
echo -e "  ssh -L 3333:127.0.0.1:3333 user@<VPS_IP>"
echo -e "  then open: https://localhost:3333"
echo ""
echo -e "${BOLD}Configured domains (phish server):${NC}"
for d in "${DOMAINS[@]}"; do
    echo -e "  • https://${d}"
done
echo ""
echo -e "${BOLD}nginx${NC}"
echo -e "  Service   : systemctl status nginx"
echo -e "  Configs   : /etc/nginx/sites-available/<domain>"
echo ""
echo -e "${BOLD}Certificates${NC}"
echo -e "  Location  : ${ACME_CERTS_DIR}/<domain>/"
echo -e "  Renewal   : manual (DNS-01) — see renewal instructions above"
echo ""
echo -e "${YELLOW}NOTE:${NC} Default Gophish credentials are printed in the service log:"
echo -e "  journalctl -u gophish | grep 'Please login'"
echo ""
