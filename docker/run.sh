#!/bin/bash
set -e

# ─── Helper ───────────────────────────────────────────────────────────────────
patch_config() {
    jq -r "$@" config.json > config.json.tmp && cat config.json.tmp > config.json
}

# ─── Admin server ─────────────────────────────────────────────────────────────
[ -n "${ADMIN_LISTEN_URL+set}" ]      && patch_config --arg v "$ADMIN_LISTEN_URL"       '.admin_server.listen_url = $v'
[ -n "${ADMIN_USE_TLS+set}" ]         && patch_config --argjson v "$ADMIN_USE_TLS"      '.admin_server.use_tls = $v'
[ -n "${ADMIN_CERT_PATH+set}" ]       && patch_config --arg v "$ADMIN_CERT_PATH"        '.admin_server.cert_path = $v'
[ -n "${ADMIN_KEY_PATH+set}" ]        && patch_config --arg v "$ADMIN_KEY_PATH"         '.admin_server.key_path = $v'
[ -n "${ADMIN_TRUSTED_ORIGINS+set}" ] && patch_config --arg v "$ADMIN_TRUSTED_ORIGINS" '.admin_server.trusted_origins = ($v|split(","))'

# ─── Phish server ─────────────────────────────────────────────────────────────
[ -n "${PHISH_LISTEN_URL+set}" ]  && patch_config --arg v "$PHISH_LISTEN_URL"  '.phish_server.listen_url = $v'
[ -n "${PHISH_USE_TLS+set}" ]     && patch_config --argjson v "$PHISH_USE_TLS" '.phish_server.use_tls = $v'
[ -n "${PHISH_CERT_PATH+set}" ]   && patch_config --arg v "$PHISH_CERT_PATH"   '.phish_server.cert_path = $v'
[ -n "${PHISH_KEY_PATH+set}" ]    && patch_config --arg v "$PHISH_KEY_PATH"    '.phish_server.key_path = $v'

# ─── Database ─────────────────────────────────────────────────────────────────
# Default: store the SQLite file inside the mounted /opt/gophish/data volume
: "${DB_FILE_PATH:=data/gophish.db}"
patch_config --arg v "$DB_FILE_PATH" '.db_path = $v'
[ -n "${DB_NAME+set}" ] && patch_config --arg v "$DB_NAME" '.db_name = $v'

# ─── Misc ─────────────────────────────────────────────────────────────────────
[ -n "${CONTACT_ADDRESS+set}" ] && patch_config --arg v "$CONTACT_ADDRESS" '.contact_address = $v'

# ─── Credential encryption (anglerphish feature) ──────────────────────────────
# The binary reads ANGLERPHISH_ENCRYPTION_KEY directly from the environment.
[ -n "${ANGLERPHISH_ENCRYPTION_KEY+set}" ] && export ANGLERPHISH_ENCRYPTION_KEY

echo "================================================"
echo "  Ultimate GoPhish – runtime configuration"
echo "================================================"
cat config.json
echo ""

exec ./gophish
