## Backend

The backend service is written in Go and provides the following functionalities:
- URL shortening
- URL redirection
- URL deletion
- Database management with PostgreSQL
- Caching with Redis

### Running the Backend

1. Ensure you have Docker and Docker Compose installed.
2. Navigate to the `backend` directory.
3. Start the services using Docker Compose:
   ```bash
   docker-compose up -d
   ```
4. Run the application: `./launch.sh`.
5. The backend service should now be running on `http://localhost:8080`.