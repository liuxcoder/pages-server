<?php

function error($code, $message) {
    http_response_code(code);
    echo("$code : $message");
    die();
}

$request_url = $_SERVER["PHP_SELF"];

if (substr($request_url, -1) == "/") {
    $request_url .= "index.html";
}

if (preg_match("/\/[a-zA-Z0-9_ +\-\/\.]+\z/", $request_url) != 1) {
    error(404, "invalid request URL '$request_url'");
}

$parts = explode("/", $request_url);
array_shift($parts); # remove empty first
$owner = strtolower(array_shift($parts));
$git_root = "/data/git/gitea-repositories/$owner/pages.git";
$file_url = implode("/", $parts);

if (!is_dir($git_root)) {
    error(404, "this user/organization does not have codeberg pages");
}

$command = "sh -c \"cd '$git_root' && /usr/bin/git show 'master:$file_url'\"";

## We are executing command twice (first for error-checking, then for actual raw output to stream),
## which seems wasteful, but it seems exec+echo cannot do raw binary output? Is this true?
exec($command, $output, $retval);
if ($retval != 0) {
    error(404 , "no such file in repo: '$file_url'");
}

$ext = pathinfo($file_url, PATHINFO_EXTENSION);
$ext = strtolower($ext);

if ($ext == "svg") {
    header("Content-Type: image/svg+xml");
} elseif ($ext == "jpg") {
    header("Content-Type: image/jpeg");
} elseif ($ext == "png") {
    header("Content-Type: image/png");
} elseif ($ext == "gif") {
    header("Content-Type: image/gif");
} elseif ($ext == "js") {
    header("Content-Type: application/javascript");
} elseif ($ext == "css") {
    header("Content-Type: text/css");
}

## If we could directly implode+echo raw output from above, we wouldn't need to execute command twice:
passthru($command);

?>

