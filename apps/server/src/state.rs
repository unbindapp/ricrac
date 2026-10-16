use std::collections::HashMap;
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

use sqlx::PgPool;

use crate::config::Config;
use crate::keys::Keys;

#[derive(Clone)]
pub struct AppState(pub Arc<Inner>);

pub struct Inner {
    pub db: PgPool,
    pub config: Config,
    pub keys: Keys,
    pub http: reqwest::Client,
    pub limiter: RateLimiter,
    /// Wakes the webhook sender when deliveries are queued.
    pub deliveries: tokio::sync::Notify,
}

impl std::ops::Deref for AppState {
    type Target = Inner;
    fn deref(&self) -> &Inner {
        &self.0
    }
}

impl AppState {
    pub fn new(db: PgPool, config: Config) -> Self {
        let keys = Keys::new(&config);
        let http = reqwest::Client::builder()
            .timeout(Duration::from_secs(15))
            .user_agent("ricrac")
            .build()
            .expect("HTTP client");
        Self(Arc::new(Inner {
            db,
            config,
            keys,
            http,
            limiter: RateLimiter::new(20, Duration::from_secs(15 * 60)),
            deliveries: tokio::sync::Notify::new(),
        }))
    }
}

/// Counts attempts per key inside a window.
pub struct RateLimiter {
    max_attempts: u32,
    window: Duration,
    entries: Mutex<HashMap<String, (u32, Instant)>>,
}

impl RateLimiter {
    pub fn new(max_attempts: u32, window: Duration) -> Self {
        Self { max_attempts, window, entries: Mutex::new(HashMap::new()) }
    }

    /// Records an attempt and says whether it went over the limit.
    pub fn hit(&self, key: &str) -> bool {
        let mut entries = self.entries.lock().unwrap();
        let now = Instant::now();
        let entry = entries.entry(key.to_string()).or_insert((0, now));
        if entry.1.elapsed() >= self.window {
            *entry = (0, now);
        }
        entry.0 += 1;
        entry.0 > self.max_attempts
    }

    pub fn prune(&self) {
        let window = self.window;
        self.entries.lock().unwrap().retain(|_, (_, since)| since.elapsed() < window);
    }
}
