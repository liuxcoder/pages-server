<?php

function send_response($code, $message = "") {
    http_response_code($code);
    echo $message;
    exit();
}

$request_uri = explode("?", $_SERVER["REQUEST_URI"])[0];
$request_url = filter_var($request_uri, FILTER_SANITIZE_URL);
$request_url = str_replace("%20", " ", $request_url);

if ($request_url === "/" and $_SERVER["HTTP_HOST"] === "codeberg.eu") {
    send_response(200, file_get_contents("./default-page.html"));
}

# Restrict allowed characters in request URI:
if (preg_match("/^\/[a-zA-Z0-9_ +\-\/\.]*\$/", $request_url) != 1) {
    send_response(404, "invalid request URL");
}

$git_prefix = "/data/git/gitea-repositories";
$parts = explode("/", $request_url);
$parts = array_diff($parts, array("")); # Remove empty parts in URL

$parts_dot = explode(".",$_SERVER["HTTP_HOST"]);
if (count($parts_dot) != 3) {
    send_response(404, "invalid subdomain");
}
$owner = $parts_dot[0];

$git_root = realpath("$git_prefix/$owner/pages.git");
$file_url = implode("/", $parts);

# Ensure that only files within $git_root are accessed:
if (substr($git_root, 0, strlen($git_prefix)) !== $git_prefix) {
    send_response(404, "this user/organization does not have codeberg pages");
}

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

if (array_key_exists($ext, $mime_types)) {
    header("Content-Type: " . $mime_types[$ext]);
} else {
    header("Content-Type: application/octet-stream");
}

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
