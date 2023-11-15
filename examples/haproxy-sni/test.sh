#!/bin/sh
if [ $# -gt 0 ]; then
  exec curl -k --resolve '*:443:127.0.0.1' "$@"
fi

fail() {
  echo "[FAIL] $@"
  exit 1
}

echo "Connecting to Gitea..."
res=$(curl https://codeberg.org -sk --resolve '*:443:127.0.0.1' --trace-ascii gitea.dump | tee /dev/stderr)
echo "$res" | grep -Fx 'Hello to Gitea!' >/dev/null || fail "Gitea didn't answer"
grep '^== Info:  issuer: O=mkcert development CA;' gitea.dump || { grep grep '^== Info:  issuer:' gitea.dump; fail "Gitea didn't use the correct certificate!"; }

echo "Connecting to Pages..."
res=$(curl https://example-page.org -sk --resolve '*:443:127.0.0.1' --trace-ascii pages.dump | tee /dev/stderr)
echo "$res" | grep -Fx 'Hello to Pages!' >/dev/null || fail "Pages didn't answer"
grep '^== Info:  issuer: CN=Caddy Local Authority\b' pages.dump || { grep '^== Info:  issuer:' pages.dump; fail "Pages didn't use the correct certificate!"; }

echo "All tests succeeded"
rm *.dump
