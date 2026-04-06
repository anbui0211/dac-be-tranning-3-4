# Upload Multiple Files APIs

## Overview

Two new APIs added for uploading multiple files to S3:

1. **`POST /upload-multiple`** - Upload specified files
2. **`POST /upload-all`** - Upload all files from assets/

---

## API 1: Upload Multiple Files

### Endpoint
```
POST /upload-multiple
Content-Type: application/json
```

### Request Body
```json
{
  "files": ["segment_01.csv", "segment_02.csv"],
  "concurrency": 5
}
```

### Request Fields
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `files` | []string | Yes | List of file names to upload |
| `concurrency` | int | No | Number of concurrent uploads (default: 5) |

### Response
```json
{
  "status": "completed",
  "total_files": 2,
  "uploaded": 2,
  "failed": 0,
  "results": [
    {
      "file": "segment_01.csv",
      "status": "success",
      "key": "segment_01.csv"
    },
    {
      "file": "segment_02.csv",
      "status": "success",
      "key": "segment_02.csv"
    }
  ],
  "duration": "1.234s"
}
```

### Response Fields
| Field | Type | Description |
|-------|------|-------------|
| `status` | string | Overall status ("completed") |
| `total_files` | int | Total files processed |
| `uploaded` | int | Successfully uploaded files |
| `failed` | int | Failed uploads |
| `results` | []UploadResult | Per-file results |
| `duration` | string | Total duration |

### Error Response
```json
{
  "error": "no files specified"
}
```

---

## API 2: Upload All Files

### Endpoint
```
POST /upload-all
Content-Type: application/json
```

### Request Body
```json
{
  "pattern": "*.csv",
  "concurrency": 5
}
```

### Request Fields
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `pattern` | string | No | Glob pattern to match files (default: "*") |
| `concurrency` | int | No | Number of concurrent uploads (default: 5) |

### Pattern Examples
| Pattern | Description |
|---------|-------------|
| `*` | All files |
| `*.csv` | All CSV files |
| `segment_*.csv` | Files starting with "segment_" and ending with ".csv" |
| `*.json` | All JSON files |

### Response
```json
{
  "status": "completed",
  "total_files": 3,
  "uploaded": 3,
  "failed": 0,
  "results": [
    {
      "file": "segment_01.csv",
      "status": "success",
      "key": "segment_01.csv"
    },
    {
      "file": "segment_02.csv",
      "status": "success",
      "key": "segment_02.csv"
    },
    {
      "file": "segment_03.csv",
      "status": "success",
      "key": "segment_03.csv"
    }
  ],
  "duration": "2.567s"
}
```

---

## Usage Examples

### Example 1: Upload Multiple Files

```bash
curl -X POST http://localhost:8081/upload-multiple \
  -H "Content-Type: application/json" \
  -d '{
    "files": ["segment_01.csv", "segment_02.csv"],
    "concurrency": 3
  }'
```

### Example 2: Upload All CSV Files

```bash
curl -X POST http://localhost:8081/upload-all \
  -H "Content-Type: application/json" \
  -d '{
    "pattern": "*.csv",
    "concurrency": 5
  }'
```

### Example 3: Upload All Files with Pattern

```bash
curl -X POST http://localhost:8081/upload-all \
  -H "Content-Type: application/json" \
  -d '{
    "pattern": "segment_*.csv",
    "concurrency": 2
  }'
```

---

## Key Features

### 1. Concurrent Uploads
- Uses worker pool pattern (similar to batch processing)
- Configurable concurrency (default = 5)
- Efficient parallel upload to S3

### 2. Error Handling
- Continues on error (individual failures don't stop entire upload)
- Collects all errors in response
- Reports summary statistics

### 3. File Discovery
- `UploadMultiple()`: Specify exact files
- `UploadAll()`: Glob pattern matching (e.g., `*.csv`, `segment_*.csv`)

### 4. Detailed Response
- Total files processed
- Success/failure counts
- Per-file results with status
- Total duration

---

## Error Handling

### Continue on Error
By default, the APIs continue uploading even if some files fail:

```json
{
  "status": "completed",
  "total_files": 3,
  "uploaded": 2,
  "failed": 1,
  "results": [
    {
      "file": "segment_01.csv",
      "status": "success",
      "key": "segment_01.csv"
    },
    {
      "file": "segment_02.csv",
      "status": "success",
      "key": "segment_02.csv"
    },
    {
      "file": "missing_file.csv",
      "status": "failed",
      "error": "failed to read file: open assets/missing_file.csv: no such file or directory"
    }
  ],
  "duration": "1.234s"
}
```

---

## Comparison with Single Upload

| Feature | `GET /upload` | `POST /upload-multiple` | `POST /upload-all` |
|---------|---------------|------------------------|-------------------|
| Files | 1 file | Multiple specified | All files by pattern |
| Method | GET | POST | POST |
| Parameters | Query params | JSON body | JSON body |
| Concurrency | Sequential | Concurrent | Concurrent |
| Error Handling | Stop on error | Continue on error | Continue on error |
| Response | Simple | Detailed | Detailed |

---

## Architecture

```
Handler Layer          Service Layer              Provider Layer
├─ UploadMultipleHandler → UploadMultiple() → S3Provider.UploadFile()
├─ UploadAllHandler     → UploadAll()      → S3Provider.UploadFile()
                       → listFilesInAssets()
                       → UploadMultiple()
                       → uploadWorker()
```

### Worker Pool Pattern

```
Files → [File Channel] → [Workers x concurrency] → [Result Channel] → Response
        ↓                           ↓                        ↓
    file1, file2, ...        uploadWorker()         Collect results
                                  ↓
                          UploadFile to S3
```

---

## Configuration

### Environment Variables
```bash
# S3 Configuration
export S3_ENDPOINT="http://localhost:9000"
export S3_ACCESS_KEY="your-access-key"
export S3_SECRET_KEY="your-secret-key"
export S3_BUCKET="your-bucket"
export AWS_REGION="us-east-1"

# Server Configuration
export PORT="8081"
```

---

## Testing

### Test Files Available
- `assets/segment_01.csv` - Empty file
- `assets/segment_02.csv` - 2 rows
- `assets/segment_03.csv` - 2 rows

### Test Scenarios

1. **Upload Multiple Files**
   ```bash
   curl -X POST http://localhost:8081/upload-multiple \
     -H "Content-Type: application/json" \
     -d '{"files": ["segment_01.csv", "segment_02.csv"]}'
   ```

2. **Upload All CSV Files**
   ```bash
   curl -X POST http://localhost:8081/upload-all \
     -H "Content-Type: application/json" \
     -d '{"pattern": "*.csv"}'
   ```

3. **Upload with Concurrency**
   ```bash
   curl -X POST http://localhost:8081/upload-multiple \
     -H "Content-Type: application/json" \
     -d '{"files": ["segment_01.csv", "segment_02.csv"], "concurrency": 2}'
   ```

4. **Upload with Pattern**
   ```bash
   curl -X POST http://localhost:8081/upload-all \
     -H "Content-Type: application/json" \
     -d '{"pattern": "segment_*.csv"}'
   ```

---

## Performance Notes

### Concurrency Impact

| Concurrency | Small Files (<1MB) | Medium Files (1-10MB) | Large Files (>10MB) |
|-------------|-------------------|---------------------|-------------------|
| 1 (Sequential) | Slow | Slow | OK |
| 5 (Default) | Fast | Fast | Good |
| 10 | Very Fast | Fast | Good |
| 20+ | Very Fast | May saturate network | May saturate network |

### Recommendations
- **Small files (<1MB)**: High concurrency (10-20)
- **Medium files (1-10MB)**: Medium concurrency (5-10)
- **Large files (>10MB)**: Low concurrency (1-5)

---

## Troubleshooting

### Common Issues

1. **"no files specified"**
   - Ensure `files` array is not empty in request

2. **"failed to list files"**
   - Check if `assets/` directory exists
   - Verify pattern is valid

3. **"failed to read file"**
   - Ensure file exists in `assets/` directory
   - Check file permissions

4. **"failed to upload to S3"**
   - Verify S3 credentials
   - Check S3 endpoint is accessible
   - Ensure bucket exists

---

## Implementation Details

### Files Modified/Created

1. **New Files**
   - `pkg/model/upload_result.go` - Response models

2. **Modified Files**
   - `pkg/services/service.go` - Added upload methods
   - `pkg/handlers/handler.go` - Added upload handlers
   - `main.go` - Added routes

### Service Methods Added
- `UploadMultiple()` - Upload multiple specified files
- `UploadAll()` - Upload all files matching pattern
- `listFilesInAssets()` - Discover files in assets/
- `uploadWorker()` - Worker for concurrent uploads

### Handler Methods Added
- `UploadMultipleHandler()` - Handle upload multiple request
- `UploadAllHandler()` - Handle upload all request

---

**Implementation Date:** 2026-04-06
**Status:** ✅ Complete & Tested
