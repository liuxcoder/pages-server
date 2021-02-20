<?php

function send_response($code, $message = "") {
    http_response_code($code);
    echo $message;
    exit();
}

$domain_parts = explode('.', $_SERVER['HTTP_HOST']);
$subdomain = implode(".", array_slice($domain_parts, 0, -2));
$tld = end($domain_parts);

$request_uri = explode("?", $_SERVER["REQUEST_URI"])[0];
$request_url = filter_var($request_uri, FILTER_SANITIZE_URL);
$request_url = str_replace("%20", " ", $request_url);
$request_url_parts = explode("/", $request_url);
$request_url_parts = array_diff($request_url_parts, array("")); # Remove empty parts in URL

$repo = "pages";

if ($tld === "org") {
    $subdomain_repo = array(
        "docs" => array("docs", "pages"),
        "fonts" => array("codeberg-fonts", "pages"),
        "get-it-on" => array("get-it-on", "pages"),
        "design" => array("Codeberg", "Design")
    );
    if (array_key_exists($subdomain, $subdomain_repo)) {
        $owner = $subdomain_repo[$subdomain][0];
        $repo = $subdomain_repo[$subdomain][1];
        // Allow CORS requests to static *.codeberg.org pages, for web fonts etc.
        header("Access-Control-Allow-Origin: *");
    } else {
        $owner = strtolower(array_shift($request_url_parts));
        if (!$owner) {
            header("Location: https://codeberg.eu");
            exit;
        }
        if (strpos($owner, ".") === false) {
            $h = "Location: https://" . $owner . ".codeberg.eu/" . implode("/", $request_url_parts);
            if ($_SERVER['QUERY_STRING'] !== "")
                $h .= "?" . $_SERVER['QUERY_STRING'];
            header($h);
            exit;
        }
    }
} else {
    $owner = strtolower($subdomain);
    if (strpos($owner, ".") !== false)
        send_response(200, "Pages not supported for user names with dots. Please rename your username to use Codeberg pages.");
    if ($owner === "raw") {
        $ch = curl_init("http://localhost:3000" . $_SERVER["REQUEST_URI"]);
        curl_setopt($ch, CURLOPT_CUSTOMREQUEST, "GET");
        curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
        curl_setopt($ch, CURLOPT_HEADER, true);
        $response = curl_exec($ch);
        $status = curl_getinfo($ch, CURLINFO_HTTP_CODE);
        $header_size = curl_getinfo($ch, CURLINFO_HEADER_SIZE);
        $header = substr($response, 0, $header_size);
        $header = explode("\r\n", $header);
        $body = substr($response, $header_size);
        foreach($header as $h) {
            if ($h && substr($h, 0, 11) != "Set-Cookie:")
            	if (substr($h, 0, 13) == "Content-Type:" && strpos($h, "text/html") !== false)
            		// text/html shouldn't be rendered on raw.codeberg.org, as it might confuse both users (with it being a legit codeberg.org subdomain) and developers (with it having a really strict CSP)
                	header(str_replace("text/html", "text/plain", $h));
                else
                	header($h);
        }
        // Allow CORS
        header("Access-Control-Allow-Origin: *");
        // Even though text/html isn't allowed, SVG files might still invoke JavaScript, which is blocked here
        header("Content-Security-Policy: default-src 'none'; style-src 'unsafe-inline'; sandbox");
        send_response($status, $body);
        die();
    }
}

$reservedUsernames = array(
    "abuse", "admin", "api", "app", "apt", "apps", "appserver", "archive", "archives", "assets", "attachments", "auth", "avatar", "avatars",
    "bbs", "blog",
    "cache", "cd", "cdn", "ci", "cloud", "cluster", "commits", "connect", "contact",
    "dashboard", "debug", "deploy", "deployment", "dev", "dns", "dns0", "dns1", "dns2", "dns3", "dns4", "doc", "download", "downloads",
    "email", "error", "explore",
    "fonts", "forum", "ftp",
    "ghost",
    "hello", "help", "helpdesk", "host",
    "i", "imap", "info", "install", "internal", "issues",
    "less", "login",
    "m", "me", "mail", "mailserver", "manifest", "merch", "merchandise", "metrics", "milestones", "mx",
    "new", "news", "notifications",
    "official", "org", "ota", "owa",
    "page", "pages", "packages", "plugins", "poll", "polls", "pop", "pop3", "portal", "postmaster", "project", "projects", "pulls",
    "raw", "remote", "repo", "robot", "robots",
    "search", "secure", "server", "shop", "shopping", "signin", "signon", "smtp", "ssl", "sso", "stars", "store", "support", "swag", "swagshop",
    "takeout", "template", "test", "testing",
    "user",
    "vote", "voting",
    "web", "webmail", "webmaster", "webshop", "webstore", "welcome", "www", "www0", "www1", "www2", "www3", "www4", "www5", "www6", "www7", "www8", "www9",
    "ns", "ns0", "ns1", "ns2", "ns3", "ns4",
    "vpn",
);

if (in_array($owner, $reservedUsernames))
    send_response(404, "Reserved user name '" . $owner . "' cannot have pages");

if (!$owner) {
    send_response(200, file_get_contents("./default-page.html"));
}

# Restrict allowed characters in request URI:
if (preg_match("/^\/[a-zA-Z0-9_ +\-\/\.]*\$/", $request_url) != 1)
    send_response(404, "invalid request URL");

$git_prefix = "/data/git/gitea-repositories";
$git_root = realpath("$git_prefix/$owner/$repo.git");
$file_url = implode("/", $request_url_parts);

# Ensure that only files within $git_root are accessed:
if (substr($git_root, 0, strlen($git_prefix)) !== $git_prefix)
    send_response(404, "this user/organization does not have codeberg pages");

# If this is a folder, we explicitly redirect to folder URL, otherwise browsers will construct invalid relative links:
$command = "sh -c \"cd '$git_root' && /usr/bin/git ls-tree 'HEAD:$file_url' > /dev/null\"";
exec($command, $output, $retval);
if ($retval === 0) {
    if (substr($request_url, -1) !== "/") {
        $h = "Location: " . $request_url . "/";
        if ($_SERVER['QUERY_STRING'] !== "")
            $h .= "?" . $_SERVER['QUERY_STRING'];
        header($h);
        exit();
    }
    if ($file_url !== "")
        $file_url .= "/";
    $file_url .= "index.html";
}

$ext = pathinfo($file_url, PATHINFO_EXTENSION);
$ext = strtolower($ext);

$mime_types = array(
    "css" => "text/css",
    "csv" => "text/csv",
    "gif" => "image/gif",
    "html" => "text/html",
    "ico" => "image/x-icon",
    "ics" => "text/calendar",
    "jpg" => "image/jpeg",
    "jpeg" => "image/jpeg",
    "js" => "application/javascript",
    "json" => "application/json",
    "pdf" => "application/pdf",
    "png" => "image/png",
    "svg" => "image/svg+xml",
    "ttf" => "font/ttf",
    "txt" => "text/plain",
    "woff" => "font/woff",
    "woff2" => "font/woff2",
    "xml" => "text/xml"
);

$mime_type = "application/octet-stream";
if (array_key_exists($ext, $mime_types))
    $mime_type = $mime_types[$ext];

header("Content-Type: " . $mime_type);

#header("Cache-Control: public, max-age=10, immutable");

$command = "sh -c \"cd '$git_root' && /usr/bin/git log --format='%H' -1\"";
exec($command, $output, $retval);
if ($retval == 0 && count($output)) {
    $revision=$output[0];
    header('ETag: "' . $revision . '"');
    if (isset($_SERVER["HTTP_IF_NONE_MATCH"])) {
        $req_revision = str_replace('"', '', str_replace('W/"', '', $_SERVER["HTTP_IF_NONE_MATCH"]));
        if ($req_revision === $revision) {
            send_response(304);
        }
    }
}

## We are executing command twice (first for send_response-checking, then for actual raw output to stream),
## which seems wasteful, but it seems exec+echo cannot do raw binary output? Is this true?
$command = "sh -c \"cd '$git_root' && /usr/bin/git show 'HEAD:$file_url'\"";
exec($command . " > /dev/null", $output, $retval);
if ($retval != 0) {
    # Try adding '.html' suffix, if this does not work either, report error
    $command = "sh -c \"cd '$git_root' && /usr/bin/git show 'HEAD:$file_url.html'\"";
    exec($command . " > /dev/null", $output, $retval);
    header("Content-Type: text/html");
    if ($retval != 0) {
        # Render user-provided 404.html if exists, generic 404 message if not:
        http_response_code(404);
        $command = "sh -c \"cd '$git_root' && /usr/bin/git show 'HEAD:404.html'\"";
        exec($command . " > /dev/null", $output, $retval);
        if ($retval != 0)
            send_response(404 , "no such file in repo: '" . htmlspecialchars($file_url) . "'");
    }
}

## If we could directly exec+echo raw output from above, we wouldn't need to execute command twice:
passthru($command);

