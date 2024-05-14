use std::{io::Error as IoError, net::SocketAddr};

use clap::Parser;
use env_logger::Env;
use futures_channel::mpsc::unbounded;
use futures_util::{future, pin_mut, stream::TryStreamExt, StreamExt};
use log::info;
use log::Level::Error;
use tokio::net::{TcpListener, TcpStream};

#[derive(Parser)]
#[command(name = "Aligned Layer Batcher")]
#[command(about = "An application with server and client subcommands", long_about = None)]
struct Cli {
    #[arg(short, long)]
    port: Option<u16>,
}

async fn handle_connection(raw_stream: TcpStream, addr: SocketAddr) {
    info!("Incoming TCP connection from: {}", addr);

    let ws_stream = tokio_tungstenite::accept_async(raw_stream)
        .await
        .expect("Error during the websocket handshake occurred");
    info!("WebSocket connection established: {}", addr);

    let (tx, rx) = unbounded();
    let (outgoing, incoming) = ws_stream.split();

    let get_incoming = incoming.try_for_each(|msg| {
        info!(
            "Received a message from {}: {}",
            addr,
            msg.to_text().unwrap()
        );
        tx.unbounded_send(msg.clone())
            .expect("Failed to send message");

        // Close channel after response
        tx.close_channel();

        future::ok(())
    });

    let receive_from_others = rx.map(Ok).forward(outgoing);
    pin_mut!(get_incoming, receive_from_others);
    future::select(get_incoming, receive_from_others).await;

    info!("{} disconnected", &addr);
}

#[tokio::main]
async fn main() -> Result<(), IoError> {
    let cli = Cli::parse();

    let port = cli.port.unwrap_or(8080);

    env_logger::Builder::from_env(Env::default().default_filter_or("info")).init();

    let addr = format!("localhost:{}", port);

    // Create the event loop and TCP listener we'll accept connections on.
    let try_socket = TcpListener::bind(&addr).await;
    let listener = try_socket.expect("Failed to bind");
    info!("Listening on: {}", addr);

    // Let's spawn the handling of each connection in a separate task.
    while let Ok((stream, addr)) = listener.accept().await {
        tokio::spawn(handle_connection(stream, addr));
    }

    Ok(())
}
