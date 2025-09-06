# Ottoscaler

## Project Overview

Ottoscaler is a Go-based Kubernetes autoscaler that dynamically manages worker pods based on Redis Streams events. It's designed for a multi-user development environment, with each developer having their own isolated Redis instance and Kind cluster. The main application runs as a "main pod" in the Kubernetes cluster, listening for scaling events and creating or deleting "worker pods" accordingly.

The core technologies used are:

*   **Go:** The main programming language.
*   **Kubernetes:** The container orchestration platform.
*   **Redis Streams:** The messaging queue for scaling events.
*   **Docker:** For containerization.
*   **Kind (Kubernetes in Docker):** For local Kubernetes clusters.
*   **gRPC:** For log streaming (to be implemented).

## Building and Running

The project uses a `Makefile` to simplify the development workflow. Here are the key commands:

*   **`make setup-user USER=<developer_name>`**: Sets up the development environment for a specific user. This command creates a dedicated Redis container, a Kind cluster, and a `.env.{developer_name}.local` file with the necessary environment variables.

*   **`make build`**: Builds the Docker image for the main application.

*   **`make deploy`**: Deploys the main application as a "main pod" to the Kind cluster.

*   **`ENV_FILE='.env.{developer_name}.local' make test-event`**: Sends a test event to the Redis stream to trigger the autoscaler.

*   **`make logs`**: Tails the logs of the main pod.

*   **`make clean`**: Cleans up all resources, including Redis containers and Kind clusters.

### Development Workflow

1.  **Set up the environment:**
    ```bash
    make setup-user USER=jinwoo
    ```

2.  **Build and deploy the application:**
    ```bash
    make build && make deploy
    ```

3.  **Send a test event:**
    ```bash
    ENV_FILE='.env.jinwoo.local' make test-event
    ```

4.  **Monitor the logs:**
    ```bash
    make logs
    ```

## Development Conventions

*   **Project Structure:** The project follows the standard Go project layout, with `cmd` for main applications, `internal` for private packages, and `pkg` for public packages.
*   **Development Environment:** Development is done by deploying the main application to a local Kind cluster. This ensures that the application is tested in a real Kubernetes environment.
*   **Multi-User Isolation:** Each developer has their own isolated environment, managed by the `scripts/setup-user-env.sh` script. This prevents conflicts between developers working on the same machine.
*   **Linting and Testing:** The project uses `golangci-lint` for linting and `go test` for testing. The `Makefile` provides `lint` and `test` targets.
*   **gRPC:** The project plans to use gRPC for log streaming from worker pods to a NestJS server. The protobuf definitions are located in the `proto` directory.
