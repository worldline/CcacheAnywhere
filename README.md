# Ccache Anywhere

## Overview

This project implements a middleware process that acts as an intermediary between ccache and various storage backends.
It's designed to handle incoming connections and forward them or process them as needed. It serves as:

- A server for ccache clients (accepting ccache requests)
- A client for storage backends (forwarding requests to the appropriate storage)

By using this mediator, ccache can interact with storage systems it doesn't natively support through a unified IPC interface.

## How It Works

```image
┌────────┐     ┌─────────────────┐     ┌──────────────────┐
│ ccache │ ◄─► │ ccache-mediator │ ◄─► │ Storage Backends │
└────────┘     └─────────────────┘     └──────────────────┘
           IPC                  API Calls
```

1. ccache communicates with the mediator through a unix socket
2. The mediator translates these requests into appropriate API calls
3. Storage operations are performed on the configured backend
4. Results are relayed back to ccache

## Supported Storage Backends

- HTTP
- Google Cloud Storage (GCS)

## Getting Started

### Prerequisites

- **Go:** You need to have Go installed on your system. You can download it from [golang.org/dl/](https://golang.org/dl/).
- **Git:** For cloning the repository.
- **C++** compiler with C++17 support for ccache
- **CMake** 3.14 or newer for ccache

### Installation

1. **Clone the repository:**

    ```bash
    git clone https://github.com/worldline/CcacheAnywhere/tree/main
    cd <repository_directory>
    ```

2. **Build the project:**
    Navigate to the project's root directory and run the build command:

    ```bash
    make build
    ```

    This will create 3 executables `ccache-backend-client`, `ccache-gs-storage` and `ccache-http-storage` in `./bin`.

3. **Install:**

   Navigate to the project's root directory and run:

   ```bash
    make install
    ```

   See the Makefile for information on installation directories and executable names.

### Running the Server

You can run the compiled executable directly. Beware that this program is not supposed to be a stand-alone process but rather should be called within the context of ccache. A Makefile is provided to help with installation, testing and building.

**Configuration environment variables:**

- **Remote URL:** Must be provided via `_CCACHE_REMOTE_URL`.
- **Socket Path:** Must be provided via `_CCACHE_SOCKET_PATH`.
- **Buffer size:** Optionally provided via `_CCACHE_BUFFER_SIZE`.
- **Attributes:**: `_CCACHE_NUM_ATTR` determines number of attributes; each is read as a pair (`_CCACHE_ATTR_KEY_i`, `_CCACHE_ATTR_VALUE_i`) for $0\leq i <$ `_CCACHE_NUM_ATTR`.

## Contributing

Contributions are welcome! Please follow these steps:

1. Fork the repository.
2. Create a new branch (`git checkout -b feature/your-feature`).
3. Make your changes.
4. Commit your changes (commit messages follow [this](https://www.conventionalcommits.org/en/v1.0.0/#specification)).
5. Push to the branch (`git push origin feature/your-feature`).
6. Submit a Pull Request.

Please ensure your code follows Go best practices and includes necessary tests.
