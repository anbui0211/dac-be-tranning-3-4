#!/bin/bash

# Script to upload CSV file to MinIO

set -e

CONFIG_FILE="minio-config.sh"

if [ ! -f "$CONFIG_FILE" ]; then
    echo "Creating MinIO config file..."
    cat > "$CONFIG_FILE" << 'EOF'
#!/bin/bash
# MinIO Configuration
MINIO_ENDPOINT="http://localhost:9000"
MINIO_ACCESS_KEY="admin"
MINIO_SECRET_KEY="password"
MINIO_BUCKET="user-data"
EOF
    chmod +x "$CONFIG_FILE"
fi

source "$CONFIG_FILE"

CSV_FILE="${1:-users_2024-04-02.csv.gz}"

if [ ! -f "$CSV_FILE" ]; then
    echo "CSV file not found: $CSV_FILE"
    echo "Please generate it first: python3 scripts/generate-csv.py"
    exit 1
fi

echo "Uploading $CSV_FILE to MinIO..."

# Create bucket if not exists
docker exec -it minio mc alias set local "$MINIO_ENDPOINT" "$MINIO_ACCESS_KEY" "$MINIO_SECRET_KEY" || true

if ! docker exec -it minio mc ls local/$MINIO_BUCKET >/dev/null 2>&1; then
    echo "Creating bucket: $MINIO_BUCKET"
    docker exec -it minio mc mb local/$MINIO_BUCKET
fi

# Upload file
docker cp "$CSV_FILE" minio:/tmp/$CSV_FILE
docker exec -it minio mc cp /tmp/$CSV_FILE local/$MINIO_BUCKET/$CSV_FILE
docker exec -it minio rm /tmp/$CSV_FILE

echo "✓ File uploaded successfully: $CSV_FILE"
echo ""
echo "Files in bucket:"
docker exec -it minio mc ls local/$MINIO_BUCKET
