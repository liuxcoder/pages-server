<?php

function send_response($code, $message) {
    http_response_code($code);
    echo $message;
    exit();
}

$request_url = filter_var($_SERVER["PHP_SELF"], FILTER_SANITIZE_URL);

if ($request_url === "/") {
    send_response(200, "
<meta name='viewport' content='width=device-width, initial-scale=1.0'>
<html>
	<head>
		<title>Codeberg Pages</title>
	</head>
	<body>
		<div style='height: 100%; display: flex; align-items: center; justify-content: center;'>
			<center>
				<h1>Codeberg Pages. Static Pages for your Projects.</h1>
				<p>Create a repo named 'pages' in your user account or org, push static content, HTML, style, fonts, images.</p>
				<p>Share your rendered content via: <pre>https://" . $_SERVER["HTTP_HOST"] . "/&lt;username&gt;/</pre></p>
				<p>Welcome to <a href='https://codeberg.org'>Codeberg.org</a>!</p>
			</center>
		</div>
	</body>
</html>
");
}

if (preg_match("/^\/[a-zA-Z0-9_ +\-\/\.]+\$/", $request_url) != 1) {
    send_response(404, "invalid request URL");
}

$git_prefix = "/data/git/gitea-repositories";
$parts = explode("/", $request_url);
array_shift($parts); # remove empty first
$owner = strtolower(array_shift($parts));
$git_root = realpath("$git_prefix/$owner/pages.git");

if (substr($git_root, 0, strlen($git_prefix)) !== $git_prefix) {
    send_response(404, "this user/organization does not have codeberg pages");
}

if (end($parts) === '') {
    array_pop($parts);
}

$file_url = implode("/", $parts);

$command = "sh -c \"cd '$git_root' && /usr/bin/git ls-tree 'master:$file_url' > /dev/null\"";
exec($command, $output, $retval);
if ($retval == 0) {
    if ($file_url == "" ) {
        $file_url = "index.html";
    } else {
	$file_url .= "/index.html";
    }
}

$ext = pathinfo($file_url, PATHINFO_EXTENSION);
$ext = strtolower($ext);

$mime_types = array(
    "svg" => "image/svg+xml",
    "png" => "image/png",
    "jpg" => "image/jpeg",
    "jpeg" => "image/jpeg",
    "gif" => "image/gif",
    "js" => "application/javascript",
    "html" => "text/html",
    "css" => "text/css",
    "ico" => "image/x-icon",
    "woff" => "font/woff",
    "woff2" => "font/woff2",
    "ttf" => "font/ttf"
);

if (array_key_exists($ext, $mime_types)) {
    $mime_type = $mime_types[$ext];
} else {
    $mime_type = "text/plain";
}

header("Content-Type: " . $mime_type);

$command = "sh -c \"cd '$git_root' && /usr/bin/git show 'master:$file_url'\"";

## We are executing command twice (first for send_response-checking, then for actual raw output to stream),
## which seems wasteful, but it seems exec+echo cannot do raw binary output? Is this true?
exec($command . " > /dev/null", $output, $retval);
if ($retval != 0) {
    # Render user-provided 404.html if exists, generic 404 message if not:
    http_response_code(404);
    header("Content-Type: text/html");
    $command = "sh -c \"cd '$git_root' && /usr/bin/git show 'master:404.html'\"";
    exec($command . " > /dev/null", $output, $retval);
    if ($retval != 0) {
        send_response(404 , "no such file in repo: '" . htmlspecialchars($file_url) . "'");
    }
}

## If we could directly exec+echo raw output from above, we wouldn't need to execute command twice:
passthru($command);

