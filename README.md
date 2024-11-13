# BitTorrent Go Implementation

This project is a BitTorrent client implementation in Go. It provides a command-line
interface (CLI) for parsing torrent files or magnet links and downloading files
from peers.

Go solution to the ["Build Your Own BitTorrent" Challenge](https://app.codecrafters.io/courses/bittorrent/overview).

**Note**: Head over to [codecrafters.io](https://codecrafters.io) to try the challenge.

## Features

- Display torrent and magnet link information
- Parse torrent files and magnet links
- Discover peers
- Download files from peers

## Installation

1. Clone the repository:

   ```sh
   git clone https://github.com/yourusername/bittorrent-go.git
   cd bittorrent-go
   ```

2. Build the project:

   ```sh
   go build -o mybittorrent ./cmd/mybittorrent
   ```

   or

   ```sh
   make build
   ```

## Usage

### Commands

- `decode <bencoded_value>`: Decode a bencoded value and print it to stdout.
- `info <torrent_file>`: Display information about a torrent file.
- `peers <torrent_file>`: Discover and display peers for a torrent file.
- `handshake <torrent_file> <peer_address>`: Perform a handshake with a peer.
- `download_piece -o <out_file> <torrent_file> <piece_idx>`: Download a specific piece of a file from peers using a torrent file.
- `download -o <out_file> <torrent_file>`: Download a file from peers using a torrent file.
- `magnet_parse <magnet_link>`: Parse and display information about a magnet link.
- `magnet_handshake <magnet_link>`: Perform a handshake with a peer using a magnet link.
- `magnet_info <magnet_link>`: Display information about a magnet link.
- `magnet_download_piece -o <out_file> <magnet_link> <piece_idx>`: Download a specific piece of a file from peers using a magnet link.
- `magnet_download -o <out_file> <magnet_link>`: Download a file from peers using a magnet link.

### Examples

- Decode a bencoded value:

  ```sh
  ./mybittorrent decode d3:cow3:moo4:spam4:eggse
  ```

- Display information about a torrent file:

  ```sh
  ./mybittorrent info example.torrent
  ```

- Discover peers for a torrent file:

  ```sh
  ./mybittorrent peers example.torrent
  ```

- Download a file using a torrent file:

  ```sh
  ./mybittorrent download -o output_file example.torrent
  ```

- Download a file using a magnet link:

  ```sh
    ./mybittorrent magnet_download -o output_file "magnet:?xt=urn:btih:..."
  ```

## Tests

To run the tests (for cases from test/test_cases_active.json), run the following:

```sh
make test
```

Or to run all tests (for cases from test/test_cases.json), run:

```sh
make test-all
```
