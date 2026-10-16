use ricrac::config::Config;
use ricrac::{AppState, app, jobs};

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| "info,sqlx=warn,tower_http=info".into()),
        )
        .init();
    let config = Config::from_env().unwrap_or_else(|err| {
        eprintln!("configuration error: {err}");
        std::process::exit(1);
    });
    if config.google().is_none() {
        tracing::warn!("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET aren't set: nobody can sign in");
    }
    let db = ricrac::connect(&config.database_url).await.expect("connect to Postgres and migrate");
    let port = config.port;
    let state = AppState::new(db, config);
    jobs::spawn(state.clone());
    let address = format!("0.0.0.0:{port}");
    let listener = tokio::net::TcpListener::bind(&address).await.expect("bind port");
    tracing::info!("ricrac {} listening on {address}", env!("CARGO_PKG_VERSION"));
    axum::serve(listener, app::router(state)).with_graceful_shutdown(stop_signal()).await.expect("server");
}

async fn stop_signal() {
    let mut terminate =
        tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate()).expect("SIGTERM handler");
    tokio::select! {
        _ = tokio::signal::ctrl_c() => {},
        _ = terminate.recv() => {},
    }
}
