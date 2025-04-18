# Real-time Ranking Microservice

## Starting the Project

This section explains how to start the real-time ranking microservice locally.

1. **Clone the Repository**:
   ```bash
   git clone <repository-url>
   cd realtime_ranking
   ```

2. **Install Dependencies**:
   ```bash
   go mod download
   ```

3. **Set Environment Variables**:
   Create a `.env` file or export variables:
   ```bash
   export REDIS_ADDRESS=localhost:6379
   export REDIS_PASSWORD=""
   export REDIS_DB=0
   ```

4. **Start Redis**:
   ```bash
   redis-server
   ```

5. **Run the Application**:
   ```bash
   go run cmd/main.go
   ```

The server will start at `http://localhost:8080`.
