# HAProxy with SNI & Host-based rules

This is a proof of concept, enabling HAProxy to use *either* SNI to redirect to backends with their own HTTPS certificates (which are then fully exposed to the client; HAProxy only proxies on a TCP level in that case), *as well as* to terminate HTTPS and use the Host header to redirect to backends that use HTTP (or a new HTTPS connection).

## How it works
1. The `http_redirect_frontend` is only there to listen on port 80 and redirect every request to HTTPS.
2. The `https_sni_frontend` listens on port 443 and chooses a backend based on the SNI hostname of the TLS connection.
3. The `https_termination_backend` passes all requests to a unix socket (using the plain TCP data).
4. The `https_termination_frontend` listens on said unix socket, terminates the HTTPS connections and then chooses a backend based on the Host header.

In the example (see [haproxy.cfg](haproxy.cfg)), the `pages_backend` is listening via HTTPS and is providing its own HTTPS certificates, while the `gitea_backend` only provides HTTP.

## How to test
```bash
docker-compose up &
./test.sh
docker-compose down

# For manual testing: all HTTPS URLs connect to localhost:443 & certificates are not verified.
./test.sh [curl-options...] <url>
```

![Screenshot of the test script's output](/attachments/c82d79ea-7586-4d4b-b340-3ad0030185d6)
