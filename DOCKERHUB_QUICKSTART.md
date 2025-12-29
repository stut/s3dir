# DockerHub Quick Start

## What You Need to Do

### 1. Get Your DockerHub Credentials

Create a DockerHub access token:
- Go to: https://hub.docker.com/settings/security
- Click "New Access Token"
- Name it: `GitHub Actions - s3dir`
- Copy the token (you won't see it again!)

### 2. Add Secrets to GitHub

Go to your GitHub repository → Settings → Secrets and variables → Actions

Add these two secrets:

| Secret Name | Value |
|-------------|-------|
| `DOCKERHUB_USERNAME` | Your DockerHub username |
| `DOCKERHUB_TOKEN` | The token you just created |

### 3. Done!

Push to GitHub and your Docker image will automatically build and publish to DockerHub!

## What Happens Automatically

✅ **On every push to main/master:**
- Runs all tests
- Builds Docker images for amd64 and arm64
- Pushes to `yourusername/s3dir:latest`

✅ **On version tags (e.g., `git tag v1.0.0`):**
- Same as above, plus:
- Pushes `yourusername/s3dir:1.0.0`
- Pushes `yourusername/s3dir:1.0`
- Pushes `yourusername/s3dir:1`

## Testing Locally

```bash
# Build
docker build -t s3dir:test .

# Run
docker run -d -p 8000:8000 -v $(pwd)/data:/data s3dir:test
```

## Creating a Release

```bash
# Tag and push
git tag v1.0.0
git push origin v1.0.0

# GitHub Actions will automatically build and publish
```

## Using Published Image

```bash
# Pull from DockerHub
docker pull yourusername/s3dir:latest

# Run
docker run -d -p 8000:8000 -v $(pwd)/data:/data yourusername/s3dir:latest
```

## Troubleshooting

**Build fails?**
1. Check GitHub Actions logs
2. Verify secrets are correct
3. Ensure DockerHub token hasn't expired

**Need help?**
See [DOCKERHUB_SETUP.md](DOCKERHUB_SETUP.md) for detailed instructions.
