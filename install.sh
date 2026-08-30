#!/usr/bin/env bash
set -euo pipefail

# Folio installer
# Usage: bash install.sh [--dir <path>]
#
# Clones folio + foliocms-theme-default, builds the binary,
# installs theme dependencies, and walks through first-run setup.

FOLIO_REPO="https://github.com/srmdn/foliocms.git"
THEME_REPO="https://github.com/srmdn/foliocms-theme-default.git"
DEFAULT_DIR="folio"
INSTALL_DIR=""

# --- helpers -----------------------------------------------------------------

info()  { printf '\033[1;34m  →\033[0m  %s\n' "$*"; }
ok()    { printf '\033[1;32m  ✓\033[0m  %s\n' "$*"; }
err()   { printf '\033[1;31m  ✗\033[0m  %s\n' "$*" >&2; }
die()   { err "$*"; exit 1; }
header(){ printf '\n\033[1m%s\033[0m\n' "$*"; }

# --- argument parsing --------------------------------------------------------

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dir) INSTALL_DIR="$2"; shift 2 ;;
    --dir=*) INSTALL_DIR="${1#--dir=}"; shift ;;
    -h|--help)
      echo "Usage: bash install.sh [--dir <path>]"
      echo "  --dir   Installation directory (default: ./$DEFAULT_DIR)"
      exit 0
      ;;
    *) die "Unknown argument: $1" ;;
  esac
done

[[ -z "$INSTALL_DIR" ]] && INSTALL_DIR="$DEFAULT_DIR"

# Resolve to absolute path
INSTALL_DIR="$(cd "$(dirname "$INSTALL_DIR")" 2>/dev/null && pwd)/$(basename "$INSTALL_DIR")" \
  || INSTALL_DIR="$(pwd)/$INSTALL_DIR"

# --- prerequisite checks -----------------------------------------------------

header "Checking prerequisites"

check_cmd() {
  command -v "$1" &>/dev/null || die "$1 is required but not found. Install it and retry."
}

check_cmd git
ok "git found"

check_cmd go
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
GO_MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
GO_MINOR=$(echo "$GO_VERSION" | cut -d. -f2)
if [[ "$GO_MAJOR" -lt 1 ]] || { [[ "$GO_MAJOR" -eq 1 ]] && [[ "$GO_MINOR" -lt 21 ]]; }; then
  die "Go 1.21 or later is required (found $GO_VERSION)"
fi
ok "Go $GO_VERSION found"

check_cmd node
NODE_VERSION=$(node --version | sed 's/v//')
NODE_MAJOR=$(echo "$NODE_VERSION" | cut -d. -f1)
if [[ "$NODE_MAJOR" -lt 22 ]]; then
  die "Node.js 22 or later is required (found $NODE_VERSION)"
fi
ok "Node.js $NODE_VERSION found"

check_cmd npm
ok "npm $(npm --version) found"

# --- install directory -------------------------------------------------------

header "Setting up install directory"

if [[ -d "$INSTALL_DIR" ]]; then
  die "Directory already exists: $INSTALL_DIR\nRemove it or choose a different --dir."
fi

info "Installing to: $INSTALL_DIR"

# --- clone folio ------------------------------------------------------------

header "Cloning Folio"

git clone --depth=1 "$FOLIO_REPO" "$INSTALL_DIR"
ok "Cloned folio"

cd "$INSTALL_DIR"

# --- build admin dashboard ---------------------------------------------------

header "Building admin dashboard"

info "Installing admin-ui dependencies..."
(cd admin-ui && npm install --silent)
info "Building admin-ui..."
(cd admin-ui && npm run build)
mkdir -p internal/adminui/dist
cp -r admin-ui/dist/. internal/adminui/dist/
ok "Admin dashboard built"

# --- build binary ------------------------------------------------------------

header "Building Folio binary"

VERSION=$(git describe --tags --exact-match 2>/dev/null || echo "dev")
info "Version: $VERSION"

go build -ldflags "-X main.version=$VERSION" -o folio ./cmd/server/
ok "Binary built: $INSTALL_DIR/folio"

# --- clone theme -------------------------------------------------------------

header "Installing default theme"

git clone --depth=1 "$THEME_REPO" theme
ok "Cloned foliocms-theme-default → theme/"

cd theme
info "Installing npm dependencies..."
npm install --silent
info "Building theme..."
npm run build
cd ..
ok "Theme built"

# --- create content dir ------------------------------------------------------

mkdir -p content/blog data
ok "Created content/blog/ and data/"

# --- create .env -------------------------------------------------------------

header "Configuring environment"

JWT_SECRET=$(openssl rand -hex 32)

printf "Port to listen on [8090]: "
read -r PORT_INPUT
PORT="${PORT_INPUT:-8090}"

cat > .env <<EOF
# Server
PORT=$PORT

# Database
DATABASE_URL=data/folio.db

# Content
CONTENT_DIR=content/blog

# Auth
JWT_SECRET=$JWT_SECRET

# Theme
THEME_DIR=theme
THEME_BUILD_CMD=npm run build
THEME_SERVICE=
EOF

ok ".env created (JWT_SECRET auto-generated)"

# --- first-run setup ---------------------------------------------------------

header "First-run setup"

info "Running ./folio --setup to create the admin account..."
echo ""
./folio --setup

# --- done --------------------------------------------------------------------

header "Installation complete"

echo ""
echo "  Start Folio:"
echo "    cd $INSTALL_DIR"
echo "    ./folio"
echo ""
echo "  The API listens on port $PORT."
echo "  Serve the theme with:  cd theme && node dist/server/entry.mjs"
echo ""
echo "  For production, run both as systemd services."
echo "  See docs/configuration.md for all options."
echo ""
