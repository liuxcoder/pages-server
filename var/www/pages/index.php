<?php

function send_response($code, $message) {
    http_response_code($code);
    echo $message;
    exit();
}

$request_url = filter_var($_SERVER["PHP_SELF"], FILTER_SANITIZE_URL);

if ($request_url === "/") {
    send_response(200, "
<html>
	<head>
		<title>Codeberg Pages</title>
	</head>
	<body>
		<h1>Codeberg Pages. Static Pages for your Project.</h1>
		<p>
			Create a repo called 'pages' to your user account or org, push static content including HTML+CSS styles, fonts, images.

		</p>
		<p>
			Access your rendered pages via:
		<pre>
			https://" . $_SERVER["HTTP_HOST"] . "/&lt;username&gt;/
		</pre>
		</p>
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
    array_shift($parts);
}

if (sizeof($parts) === 0 || strpos(end($parts), ".") === false) {
    $h = "Location: https://" . $_SERVER["HTTP_HOST"] . $_SERVER["REQUEST_URI"];
    if (substr($h, -1) != "/") {
        $h .= "/";
    }
    $h .= "index.html";
    header($h);
    exit();
}

$file_url = implode("/", $parts);

$command = "sh -c \"cd '$git_root' && /usr/bin/git show 'master:$file_url'\"";

## We are executing command twice (first for send_response-checking, then for actual raw output to stream),
## which seems wasteful, but it seems exec+echo cannot do raw binary output? Is this true?
exec($command, $output, $retval);
if ($retval != 0) {
    send_response(404 , "no such file in repo: '" . htmlspecialchars($file_url) . "'");
}

$mime_types = array(
    "svg" => "image/svg+xml",
    "png" => "image/png",
    "jpg" => "image/jpeg",
    "jpeg" => "image/jpeg",
    "gif" => "image/gif",
    "js" => "application/javascript",
    "html" => "text/html",
    "css" => "text/css",
    "ico" => "image/x-icon"
);

$ext = pathinfo($file_url, PATHINFO_EXTENSION);
$ext = strtolower($ext);

if (array_key_exists($ext, $mime_types)) {
    $mime_type = $mime_types[$ext];
} else {
    $mime_type = "text/plain";
}

header("Content-Type: " . $mime_type);

## If we could directly implode+echo raw output from above, we wouldn't need to execute command twice:
passthru($command);

?>

