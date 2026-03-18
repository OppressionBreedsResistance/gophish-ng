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
    git curl wget socat nginx \
    build-essential ca-certificates \
    software-properties-common

success "Packages installed."

# =============================================================================
# 2. Go
# =============================================================================
step "Checking Go installation"

GO_MIN_VERSION="1.21"

install_go() {
    info "Fetching latest stable Go..."
    GO_LATEST=$(curl -s https://go.dev/VERSION?m=text | head -1)
    GO_TAR="${GO_LATEST}.linux-amd64.tar.gz"
    GO_URL="https://golang.org/dl/${GO_TAR}"
    info "Downloading ${GO_TAR}..."
    wget -q --show-progress "${GO_URL}" -O /tmp/go.tar.gz
    rm -rf /usr/local/go
    tar -C /usr/local -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz
    export PATH="$PATH:/usr/local/go/bin"
    success "Go ${GO_LATEST} installed."
}

if command -v go &>/dev/null; then
    CURRENT_GO=$(go version | awk '{print $3}' | sed 's/go//')
    info "Found Go ${CURRENT_GO}"
    # Simple major.minor check
    CURRENT_MAJOR=$(echo "$CURRENT_GO" | cut -d. -f1)
    CURRENT_MINOR=$(echo "$CURRENT_GO" | cut -d. -f2)
    MIN_MINOR=$(echo "$GO_MIN_VERSION" | cut -d. -f2)
    if (( CURRENT_MAJOR < 1 || CURRENT_MINOR < MIN_MINOR )); then
        warn "Go version too old, upgrading..."
        install_go
    else
        success "Go version OK."
    fi
else
    install_go
fi

export PATH="$PATH:/usr/local/go/bin"

# =============================================================================
# 3. acme.sh
# =============================================================================
step "Checking acme.sh"

if [[ -f "${ACME_HOME}/acme.sh" ]]; then
    success "acme.sh already installed."
else
    info "Installing acme.sh..."
    curl -fsSL https://get.acme.sh | sh -s -- --home "${ACME_HOME}" --email "${ACME_EMAIL}" --no-cron
    success "acme.sh installed."
fi

ACME="${ACME_HOME}/acme.sh"

# =============================================================================
# 4. Prepare webroot for HTTP-01 challenge
# =============================================================================
step "Preparing ACME webroot"

mkdir -p "${WEBROOT}/.well-known/acme-challenge"
chown -R www-data:www-data "${WEBROOT}" 2>/dev/null || true

# Temporary nginx config: serve /.well-known on port 80 for all domains
cat > /etc/nginx/sites-available/_acme-challenge <<'NGINXEOF'
server {
    listen 80 default_server;
    server_name _;

    location /.well-known/acme-challenge/ {
        root /var/www/acme-challenge;
    }

    location / {
        return 301 https://$host$request_uri;
    }
}
NGINXEOF

# Disable default site, enable challenge site
rm -f /etc/nginx/sites-enabled/default
ln -sf /etc/nginx/sites-available/_acme-challenge /etc/nginx/sites-enabled/_acme-challenge

nginx -t && systemctl reload nginx
success "nginx ready for ACME challenges."

# =============================================================================
# 5. Issue TLS certificates
# =============================================================================
step "Issuing TLS certificates"

mkdir -p "${ACME_CERTS_DIR}"

declare -A CERT_PATHS=()
declare -A KEY_PATHS=()

for domain in "${DOMAINS[@]}"; do
    info "Issuing certificate for ${domain}..."

    CERT_OUT="${ACME_CERTS_DIR}/${domain}"
    mkdir -p "${CERT_OUT}"

    "${ACME}" --issue \
        --home "${ACME_HOME}" \
        -d "${domain}" \
        --webroot "${WEBROOT}" \
        --keylength ec-256 \
        --force \
        || warn "Certificate issue for ${domain} returned non-zero — may already exist, continuing."

    "${ACME}" --install-cert \
        --home "${ACME_HOME}" \
        -d "${domain}" \
        --ecc \
        --fullchain-file "${CERT_OUT}/fullchain.cer" \
        --key-file       "${CERT_OUT}/key.pem" \
        --reloadcmd      "systemctl reload nginx"

    CERT_PATHS[$domain]="${CERT_OUT}/fullchain.cer"
    KEY_PATHS[$domain]="${CERT_OUT}/key.pem"
    success "Certificate installed for ${domain}."
done

# =============================================================================
# 6. nginx — final site configs
# =============================================================================
step "Writing nginx reverse-proxy configs"

# Remove temporary ACME-only config
rm -f /etc/nginx/sites-enabled/_acme-challenge
rm -f /etc/nginx/sites-available/_acme-challenge

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
# 8. config.json
# =============================================================================
step "Writing config.json"

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
    }
}
JSONEOF

success "config.json written."

# =============================================================================
# 9. Dedicated system user
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
# 10. systemd service
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
# 11. Auto-renew cron for acme.sh
# =============================================================================
step "Setting up certificate auto-renewal"

CRON_JOB="0 3 * * * ${ACME} --cron --home ${ACME_HOME} --reloadcmd 'systemctl reload nginx' >> /var/log/acme-renew.log 2>&1"
( crontab -l 2>/dev/null | grep -v "acme.sh" ; echo "${CRON_JOB}" ) | crontab -
success "Daily renewal cron set (runs at 03:00)."

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
echo -e "  Renewal   : automatic via cron (daily at 03:00)"
echo ""
echo -e "${YELLOW}NOTE:${NC} Default Gophish credentials are printed in the service log:"
echo -e "  journalctl -u gophish | grep 'Please login'"
echo ""
