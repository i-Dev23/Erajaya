# PostgreSQL Docker Setup

Setup PostgreSQL untuk PPS Tokopedia Service dengan Docker.

## Quick Start

### 1. Build dan Start PostgreSQL

```bash
# Masuk ke folder database
cd database

# Build dan start container
docker-compose up -d

# Check logs
docker-compose logs -f
```

### 2. Verify Installation

```bash
# Check container status
docker ps | grep pps-postgres

# Test connection dari local
docker exec -it pps-postgres psql -U pps-devl -d pps-tokopedia -c "SELECT 1;"

# List all tables
docker exec -it pps-postgres psql -U pps-devl -d pps-tokopedia -c "\dt"

# Check database size
docker exec -it pps-postgres psql -U pps-devl -d pps-tokopedia -c "SELECT pg_size_pretty(pg_database_size('pps-tokopedia'));"
```

### 3. Test Connection dari Server Lain

```bash
# Install PostgreSQL client di server lain
sudo apt install postgresql-client -y

# Test connection (ganti <SERVER_IP> dengan IP server PostgreSQL)
psql -h <SERVER_IP> -p 5432 -U pps-devl -d pps-tokopedia

# Atau test dengan Docker
docker run --rm postgres:15-alpine psql -h <SERVER_IP> -p 5432 -U pps-devl -d pps-tokopedia
```

## Configuration

### Database Credentials

- **Database**: pps-tokopedia
- **Username**: pps-devl
- **Password**: pps123**
- **Port**: 5432

### Connection String

```
postgres://pps-devl:pps123**@<SERVER_IP>:5432/pps-tokopedia
```

### Environment Variables untuk Golang Service

```yaml
POSTGRES_DSN: postgres://pps-devl:pps123**@<SERVER_IP>:5432/pps-tokopedia
POSTGRES_MAX_CONNS: 10
POSTGRES_MIN_CONNS: 5
POSTGRES_MAX_CONN_LIFETIME: 30m
```

## Database Schemas

Container akan otomatis execute semua schema saat first run:

1. `callback_log_schema.sql` - Callback logging table
2. `inquiry_schema.sql` - Inquiry transaction table
3. `log_schema.sql` - HTTP request/response logging
4. `maintenance_schema.sql` - Partition maintenance tables
5. `mapping_schema.sql` - Error code mapping table
6. `payment_schema.sql` - Payment transaction table

## Performance Tuning

PostgreSQL sudah di-tune dengan parameter optimal untuk 50-100 TPS:

```
max_connections=100
shared_buffers=256MB
effective_cache_size=1GB
work_mem=4MB
maintenance_work_mem=64MB
```

## Data Persistence

Data PostgreSQL disimpan di Docker volume:

```bash
# Check volume
docker volume ls | grep postgres

# Inspect volume
docker volume inspect database_postgres_data

# Backup volume
docker run --rm -v database_postgres_data:/data -v $(pwd):/backup alpine tar czf /backup/postgres_backup.tar.gz /data
```

## Backup & Restore

### Manual Backup

```bash
# Backup all databases
docker exec pps-postgres pg_dumpall -U pps-devl > backup_all.sql

# Backup specific database
docker exec pps-postgres pg_dump -U pps-devl pps-tokopedia > backup_pps_tokopedia.sql

# Backup dengan compression
docker exec pps-postgres pg_dump -U pps-devl -Fc pps-tokopedia > backup_pps_tokopedia.dump
```

### Restore

```bash
# Restore all databases
docker exec -i pps-postgres psql -U pps-devl < backup_all.sql

# Restore specific database
docker exec -i pps-postgres psql -U pps-devl pps-tokopedia < backup_pps_tokopedia.sql

# Restore dari compressed dump
docker exec -i pps-postgres pg_restore -U pps-devl -d pps-tokopedia backup_pps_tokopedia.dump
```

## Monitoring

### Check Active Connections

```bash
docker exec pps-postgres psql -U pps-devl -d pps-tokopedia -c "
SELECT 
    datname,
    count(*) as connections,
    max(state) as state
FROM pg_stat_activity 
WHERE datname = 'pps-tokopedia'
GROUP BY datname;"
```

### Check Slow Queries

```bash
docker exec pps-postgres psql -U pps-devl -d pps-tokopedia -c "
SELECT 
    pid,
    now() - query_start as duration,
    state,
    query
FROM pg_stat_activity
WHERE state != 'idle'
ORDER BY duration DESC
LIMIT 10;"
```

### Check Database Size

```bash
docker exec pps-postgres psql -U pps-devl -d pps-tokopedia -c "
SELECT 
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC
LIMIT 10;"
```

## Troubleshooting

### Cannot Connect from Other Servers

1. Check firewall:
```bash
sudo ufw allow 5432/tcp
```

2. Verify PostgreSQL listening:
```bash
docker exec pps-postgres psql -U pps-devl -c "SHOW listen_addresses;"
# Should return: *
```

3. Test port accessibility:
```bash
# From other server
telnet <SERVER_IP> 5432
# Or
nc -zv <SERVER_IP> 5432
```

### Container Restart Issues

```bash
# Check logs
docker-compose logs postgres

# Check container status
docker inspect pps-postgres

# Restart container
docker-compose restart postgres
```

### Performance Issues

```bash
# Check current settings
docker exec pps-postgres psql -U pps-devl -c "SHOW ALL;"

# Analyze slow queries
docker exec pps-postgres psql -U pps-devl -d pps-tokopedia -c "
SELECT query, calls, total_time, mean_time 
FROM pg_stat_statements 
ORDER BY mean_time DESC 
LIMIT 10;"
```

## Cleanup

### Stop Container

```bash
docker-compose down
```

### Remove Container dan Volume

```bash
# WARNING: This will delete all data!
docker-compose down -v
```

### Remove Image

```bash
docker rmi pps-postgres:1.0.0
```

## Security Notes

⚠️ **IMPORTANT**: 

1. Change default password di production:
   ```yaml
   POSTGRES_PASSWORD: <strong_password>
   ```

2. Restrict network access dengan firewall rules

3. Gunakan SSL/TLS untuk production:
   ```
   - "-c"
   - "ssl=on"
   - "-c"
   - "ssl_cert_file=/path/to/cert.pem"
   ```

4. Consider menggunakan `pg_hba.conf` untuk restrict access per IP:
   ```
   # Create custom pg_hba.conf
   host all all 192.168.3.0/24 md5
   ```

## Production Checklist

- [ ] Change default password
- [ ] Configure firewall rules
- [ ] Setup automated backups
- [ ] Enable SSL/TLS
- [ ] Configure monitoring (Prometheus/Grafana)
- [ ] Setup log rotation
- [ ] Configure connection pooling (PgBouncer recommended untuk >100 TPS)
- [ ] Test disaster recovery procedure
