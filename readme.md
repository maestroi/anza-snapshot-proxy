# Snapshot Proxy Service

This project is a proxy service for handling requests related to snapshots and incremental snapshots. It listens for incoming requests on port `14705` and forwards them to a configured snapshot proxy (e.g., localhost:8899). The service supports both file downloads and JSON-RPC requests.


## Prerequisites

- Docker
- Docker Compose

Make sure Docker and Docker Compose are installed on your system. You can check if they are installed by running:

```bash
docker --version
docker-compose --version

Project Structure
	•	Dockerfile: A multi-stage Dockerfile that builds and runs the Go application.
	•	docker-compose.yml: A Docker Compose file that sets up the service and exposes the required port.
	•	config.json: Configuration file for whitelist and blacklist of methods (create this file as per your needs).
	•	main.go: Go application that handles HTTP requests and forwards them to the snapshot proxy.

Setup and Installation

Docker
	1.	Clone the repository:

git clone <repository-url>
cd <repository-directory>


	2.	Build the Docker image:

docker build -t snapshot-proxy .


	3.	Run the Docker container:

docker run -p 14705:14705 snapshot-proxy



Docker Compose

Docker Compose simplifies the process of managing multiple containers. To use Docker Compose:
	1.	Clone the repository if you haven’t already:

git clone <repository-url>
cd <repository-directory>


	2.	Create the config.json file in the project directory with your desired whitelist and blacklist configurations.
	3.	Build and start the service using Docker Compose:

docker-compose up --build


	4.	The service will be accessible at http://localhost:14705 (or your host’s IP address).

Usage

Once the service is running, it will:
	•	Forward requests on port 14705 to a snapshot proxy.
	•	Support file download requests (e.g., genesis.tar.bz2 and snapshot-related files).
	•	Handle JSON-RPC requests and forward them to the configured snapshot proxy.
	•	Support both GET and POST methods.

To interact with the service, you can send HTTP requests to http://localhost:14705 for various actions, such as downloading snapshots or making JSON-RPC calls.

Example Request (JSON-RPC)

Here’s an example of a JSON-RPC request:

{
  "jsonrpc": "2.0",
  "method": "getSnapshotStatus",
  "params": [],
  "id": 1
}

Example Request (File Download)

To download a snapshot file (e.g., snapshot-2022.tar.bz2), send a GET request:

curl -X GET http://localhost:14705/snapshot-2022.tar.bz2

Configuration

The service can be configured by modifying the config.json file, where you can specify which methods should be allowed or blocked.

Example config.json:

{
  "whitelisted_methods": ["getSnapshotStatus", "getBlockHeight"],
  "blacklisted_methods": ["getInvalidMethod"]
}

	•	whitelisted_methods: List of allowed methods for JSON-RPC requests.
	•	blacklisted_methods: List of blocked methods for JSON-RPC requests.

License

This project is licensed under the MIT License - see the LICENSE file for details.
