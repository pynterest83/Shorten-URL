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
3. Navigate to `Redis` directory. Start the services using Docker Compose:
   ```bash
   docker-compose up -d
   ```
4. Run the command to initialize Redis:
   ```bash  
   docker exec -it redis_7000 redis-cli --cluster create redis_7000:7000 redis_7001:7001 redis_7002:7002 redis_7100:7100 redis_7101:7101 redis_7102:7102 --cluster-replicas 1 --cluster-yes
   ```
5. Navigate back to the `backend` directory.
5. Run the command to initialize the database:
   ```bash
   docker exec -it postgres_container psql -U shortenurl -d shortenurl
   CREATE TABLE urls (
   id VARCHAR PRIMARY KEY,
   url TEXT NOT NULL
   );
   CREATE INDEX idx_urls_url_shard_0 ON urls_shard_0 (url);
   CREATE INDEX idx_urls_url_shard_1 ON urls_shard_1 (url);
   CREATE INDEX idx_urls_url_shard_2 ON urls_shard_2 (url);
   ```
6. Run the application: `./launch.sh`.
7. The backend service should now be running on `http://localhost:8080`.

![Screenshot 2024-11-24 112120](https://github.com/user-attachments/assets/b5ebba6b-4ae1-422d-901f-e5b1f272d915)
