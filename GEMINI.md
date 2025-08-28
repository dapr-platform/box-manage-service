# GEMINI.md

## Project Overview

This project is the backend service for an AI Box Management System. It's written in Go and provides a RESTful API for managing AI boxes, models, tasks, and videos.

The project is designed to be run with [Dapr](https://dapr.io/), an open-source, portable, event-driven runtime that makes it easy for developers to build resilient, microservice stateless and stateful applications that run on the cloud and edge.

Key technologies used:

*   **Programming Language:** Go
*   **Web Framework:** Chi
*   **Database:** PostgreSQL
*   **ORM:** GORM
*   **API Documentation:** Swagger
*   **Containerization:** Docker
*   **Microservices Runtime:** Dapr

The application is structured into several layers:

*   **`api`:** Contains the API routes and controllers.
*   **`models`:** Defines the data models (database schema).
*   **`service`:** Implements the business logic.
*   **`repository`:** Provides a data access layer.
*   **`config`:** Handles application configuration.

## Building and Running

### Prerequisites

*   Go (version 1.23.1 or later)
*   Docker
*   Dapr CLI

### Running the Service

1.  **Install Dependencies:**

    ```bash
    go mod tidy
    ```

2.  **Run with Dapr:**

    The application is intended to be run with Dapr. The following command will start the application with the Dapr sidecar.

    ```bash
    dapr run --app-id box-manage-service --app-port 8080 -- go run main.go
    ```

### Building and Running with Docker

The project includes a `Dockerfile` for building a Docker image.

1.  **Build the Docker image:**

    ```bash
    docker build -t box-manage-service .
    ```

2.  **Run the Docker container:**

    ```bash
    docker run -p 8080:8080 box-manage-service
    ```

## Development Conventions

*   **API:** The project follows RESTful API design principles.
*   **Configuration:** All configuration is managed through environment variables. See `config/config.go` for a full list of available options.
*   **Database Migrations:** Database migrations are automatically run on application startup.
*   **API Documentation:** API documentation is generated using Swagger. It can be accessed at `/swagger/index.html`.
*   **Testing:** The `tests` directory contains shell scripts for testing various parts of the application.
*   **Code Style:** The code follows standard Go formatting and conventions.
