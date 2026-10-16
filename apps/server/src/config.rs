#[derive(Clone)]
pub struct Config {
    pub port: u16,
    pub database_url: String,
    /// External base URL, e.g. https://ricrac.example.com. Used for sign-in, OAuth metadata, MCP
    /// host checks and the links the API returns.
    pub public_url: String,
    /// Signs session cookies. Changing it signs everyone out.
    pub secret_key: String,
    /// Sign-in with Google. Without them nobody can sign in, but tokens keep working.
    pub google_client_id: Option<String>,
    pub google_client_secret: Option<String>,
    /// Largest upload accepted, in bytes.
    pub max_upload_size: usize,
    /// Force the `Secure` flag on cookies. When unset it is derived from `X-Forwarded-Proto`.
    pub secure_cookies: Option<bool>,
}

impl std::fmt::Debug for Config {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Config").field("public_url", &self.public_url).finish_non_exhaustive()
    }
}

impl Config {
    pub fn from_env() -> Result<Self, String> {
        let public_url = env("PUBLIC_URL").ok_or("PUBLIC_URL must be set, e.g. https://ricrac.example.com")?;
        let public_url = public_url.trim_end_matches('/').to_string();
        if !(public_url.starts_with("https://") || public_url.starts_with("http://")) {
            return Err("PUBLIC_URL must start with https:// or http://".into());
        }
        let secret_key = env("SECRET_KEY").ok_or("SECRET_KEY must be set to a long random string")?;
        if secret_key.len() < 32 {
            return Err("SECRET_KEY must be at least 32 characters".into());
        }
        let port = match env("PORT") {
            Some(port) => port.parse().map_err(|_| "PORT must be a number")?,
            None => 3000,
        };
        let max_upload_mb: usize = match env("MAX_UPLOAD_MB") {
            Some(mb) => mb.parse().map_err(|_| "MAX_UPLOAD_MB must be a number")?,
            None => 25,
        };
        let secure_cookies = match env("SECURE_COOKIES").as_deref() {
            Some("true" | "1") => Some(true),
            Some("false" | "0") => Some(false),
            _ => None,
        };
        Ok(Self {
            port,
            database_url: env("DATABASE_URL").ok_or("DATABASE_URL must be set")?,
            public_url,
            secret_key,
            google_client_id: env("GOOGLE_CLIENT_ID"),
            google_client_secret: env("GOOGLE_CLIENT_SECRET"),
            max_upload_size: max_upload_mb * 1024 * 1024,
            secure_cookies,
        })
    }

    pub fn google(&self) -> Option<(&str, &str)> {
        Some((self.google_client_id.as_deref()?, self.google_client_secret.as_deref()?))
    }

    /// Cookie names on https carry the `__Host-` prefix, so no other host can set them.
    pub fn cookie_name(&self, name: &str) -> String {
        if self.public_url.starts_with("https://") { format!("__Host-{name}") } else { name.to_string() }
    }

    pub fn mcp_url(&self) -> String {
        format!("{}/mcp", self.public_url)
    }
}

fn env(name: &str) -> Option<String> {
    std::env::var(name).ok().map(|v| v.trim().to_string()).filter(|v| !v.is_empty())
}
