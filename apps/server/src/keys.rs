//! Signed values (session cookies, the sign-in round trip) and the secrets handed to clients.

use axum::http::HeaderMap;
use axum::http::header::COOKIE;
use base64::Engine;
use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use chrono::Utc;
use hmac::{Hmac, Mac};
use rand::RngCore;
use sha2::{Digest, Sha256};
use subtle::ConstantTimeEq;
use uuid::Uuid;

use crate::config::Config;

type HmacSha256 = Hmac<Sha256>;

pub const SESSION_TTL: i64 = 30 * 24 * 3600;

pub struct Keys {
    secret: [u8; 32],
    pub session_cookie: String,
}

impl Keys {
    pub fn new(config: &Config) -> Self {
        Self {
            secret: Sha256::digest(config.secret_key.as_bytes()).into(),
            session_cookie: config.cookie_name("rr_session"),
        }
    }

    fn mac(&self, data: &[u8]) -> Vec<u8> {
        let mut mac = HmacSha256::new_from_slice(&self.secret).expect("hmac accepts any key size");
        mac.update(data);
        mac.finalize().into_bytes().to_vec()
    }

    pub fn sign(&self, payload: &str) -> String {
        format!("{}.{}", URL_SAFE_NO_PAD.encode(payload), URL_SAFE_NO_PAD.encode(self.mac(payload.as_bytes())))
    }

    pub fn verify(&self, token: &str) -> Option<String> {
        let (payload_b64, sig_b64) = token.split_once('.')?;
        let payload = String::from_utf8(URL_SAFE_NO_PAD.decode(payload_b64).ok()?).ok()?;
        let sig = URL_SAFE_NO_PAD.decode(sig_b64).ok()?;
        bool::from(self.mac(payload.as_bytes()).ct_eq(&sig)).then_some(payload)
    }

    pub fn session_token(&self, user: Uuid) -> String {
        self.sign(&format!("user:{}:{user}", Utc::now().timestamp() + SESSION_TTL))
    }

    pub fn session_user(&self, headers: &HeaderMap) -> Option<Uuid> {
        let payload = self.verify(&get_cookie(headers, &self.session_cookie)?)?;
        let (exp, user) = payload.strip_prefix("user:")?.split_once(':')?;
        (exp.parse::<i64>().ok()? > Utc::now().timestamp()).then_some(())?;
        user.parse().ok()
    }
}

pub fn get_cookie(headers: &HeaderMap, name: &str) -> Option<String> {
    headers
        .get_all(COOKIE)
        .iter()
        .filter_map(|v| v.to_str().ok())
        .flat_map(|v| v.split(';'))
        .filter_map(|pair| pair.trim().split_once('='))
        .find(|(k, _)| *k == name)
        .map(|(_, v)| v.to_string())
}

pub fn is_secure_request(config: &Config, headers: &HeaderMap) -> bool {
    config.secure_cookies.unwrap_or_else(|| {
        headers
            .get("x-forwarded-proto")
            .and_then(|v| v.to_str().ok())
            .is_some_and(|v| v.split(',').next().is_some_and(|p| p.trim().eq_ignore_ascii_case("https")))
    })
}

pub fn build_cookie(name: &str, value: &str, max_age: i64, secure: bool) -> String {
    format!(
        "{name}={value}; Path=/; Max-Age={}; HttpOnly; SameSite=Lax{}",
        max_age.max(0),
        if secure { "; Secure" } else { "" }
    )
}

pub fn client_ip(headers: &HeaderMap) -> String {
    ["cf-connecting-ip", "x-real-ip", "x-forwarded-for"]
        .iter()
        .find_map(|name| {
            let value = headers.get(*name)?.to_str().ok()?.split(',').next()?.trim();
            (!value.is_empty()).then(|| value.to_string())
        })
        .unwrap_or_else(|| "unknown".into())
}

/// A random URL-safe secret with a readable prefix, e.g. `rr_at_...`.
pub fn random_secret(prefix: &str) -> String {
    let mut bytes = [0u8; 32];
    rand::thread_rng().fill_bytes(&mut bytes);
    format!("{prefix}{}", URL_SAFE_NO_PAD.encode(bytes))
}

/// Secrets (tokens, codes, keys, client secrets) are only stored as SHA-256 hashes.
pub fn secret_hash(secret: &str) -> String {
    hex::encode(Sha256::digest(secret.as_bytes()))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn signed_values_round_trip_and_reject_tampering() {
        let keys = Keys { secret: [7; 32], session_cookie: "rr_session".into() };
        let token = keys.sign("hello:world");
        assert_eq!(keys.verify(&token).as_deref(), Some("hello:world"));
        let forged = format!("{}.{}", URL_SAFE_NO_PAD.encode("hello:admin"), token.split_once('.').unwrap().1);
        assert_eq!(keys.verify(&forged), None);
    }
}
