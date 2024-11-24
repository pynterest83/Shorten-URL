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

![Screenshot 2024-11-24 112120](https://github.com/user-attachments/assets/b5ebba6b-4ae1-422d-901f-e5b1f272d915)
