# DockerHub Setup Guide

This guide explains how to configure GitHub Actions to automatically build and push Docker images to DockerHub.

## Prerequisites

1. A DockerHub account ([sign up here](https://hub.docker.com/signup))
2. A GitHub repository with this project
3. GitHub repository admin access

## Step-by-Step Setup

### 1. Create DockerHub Access Token

1. Log in to [DockerHub](https://hub.docker.com/)
2. Click on your username in the top-right corner
3. Select **Account Settings**
4. Click on **Security** in the left sidebar
5. Click **New Access Token**
6. Configure the token:
   - **Description**: `GitHub Actions - s3dir`
   - **Access permissions**: Select **Read, Write, Delete**
7. Click **Generate**
8. **IMPORTANT**: Copy the token immediately (it won't be shown again)

### 2. Add Secrets to GitHub Repository

1. Go to your GitHub repository
2. Click on **Settings** tab
3. In the left sidebar, click **Secrets and variables** → **Actions**
4. Click **New repository secret**
5. Add the following secrets:

#### Secret 1: DOCKERHUB_USERNAME
- **Name**: `DOCKERHUB_USERNAME`
- **Value**: Your DockerHub username (e.g., `yourusername`)
- Click **Add secret**

#### Secret 2: DOCKERHUB_TOKEN
- **Name**: `DOCKERHUB_TOKEN`
- **Value**: The access token you copied in Step 1
- Click **Add secret**

### 3. Create DockerHub Repository (Optional)

The workflow will push to: `yourusername/s3dir`

You can either:
- **Let GitHub Actions create it automatically** (recommended)
- **Create it manually**:
  1. Go to [DockerHub](https://hub.docker.com/)
  2. Click **Create Repository**
  3. Name: `s3dir`
  4. Visibility: Public or Private
  5. Click **Create**

### 4. Verify Setup

After pushing to GitHub, the workflow will:
1. Run all tests
2. Build multi-platform Docker images (amd64, arm64)
3. Push to DockerHub with appropriate tags

### 5. Workflow Triggers

The Docker build workflow triggers on:

- **Push to main/master branch** → Builds with `latest` tag
- **Push tag matching `v*.*.*`** → Builds with version tags (e.g., `v1.0.0`, `1.0`, `1`)
- **Manual trigger** → Via GitHub Actions UI

## Image Tags

The workflow automatically creates multiple tags:

### For branch pushes:
- `latest` (for main/master branch)
- `main` or `master` (branch name)
- `main-<commit-sha>` (branch + commit)

### For version tags (e.g., `v1.2.3`):
- `1.2.3` (full version)
- `1.2` (major.minor)
- `1` (major only)
- `latest` (if on default branch)

## Example: Creating a Release

To create a versioned release:

```bash
# Tag your commit
git tag v1.0.0
git push origin v1.0.0
```

This will build and push:
- `yourusername/s3dir:1.0.0`
- `yourusername/s3dir:1.0`
- `yourusername/s3dir:1`
- `yourusername/s3dir:latest`

## Using the Published Image

Once published, users can pull and run your image:

```bash
# Pull the image
docker pull yourusername/s3dir:latest

# Run the container
docker run -d \
  -p 8000:8000 \
  -v $(pwd)/data:/data \
  --name s3dir \
  yourusername/s3dir:latest
```

## Troubleshooting

### Workflow Fails with "Authentication Required"

**Problem**: GitHub Actions can't authenticate with DockerHub

**Solutions**:
1. Verify `DOCKERHUB_USERNAME` secret is correct (case-sensitive)
2. Verify `DOCKERHUB_TOKEN` is valid and not expired
3. Regenerate the DockerHub access token if needed
4. Ensure secrets are added to the correct repository

### Workflow Fails with "Repository Not Found"

**Problem**: DockerHub repository doesn't exist or is inaccessible

**Solutions**:
1. Create the repository manually on DockerHub
2. Ensure the repository name matches: `yourusername/s3dir`
3. Check repository visibility settings

### Build Fails on Specific Platform

**Problem**: Build fails for `linux/arm64` or another platform

**Solutions**:
1. Check the Dockerfile for platform-specific issues
2. Review build logs in GitHub Actions
3. Test locally with: `docker buildx build --platform linux/arm64 .`

## Manual Testing

Test the workflow locally using Docker Buildx:

```bash
# Set up buildx
docker buildx create --use

# Build multi-platform image
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t yourusername/s3dir:test \
  --push \
  .
```

## Customization

### Change Docker Image Name

Edit `.github/workflows/docker-publish.yml`:

```yaml
env:
  DOCKER_IMAGE_NAME: your-custom-name  # Change this
```

### Add Additional Tags

Edit the `tags:` section in `docker-publish.yml`:

```yaml
tags: |
  type=raw,value=stable
  type=raw,value=prod
  # Add your custom tags here
```

### Build for Additional Platforms

Edit the `platforms:` in `docker-publish.yml`:

```yaml
platforms: linux/amd64,linux/arm64,linux/arm/v7
```

## Security Best Practices

1. **Use Access Tokens**: Never use your DockerHub password in GitHub secrets
2. **Limit Token Scope**: Create tokens with minimum required permissions
3. **Rotate Tokens**: Regenerate tokens periodically
4. **Monitor Usage**: Check DockerHub for unauthorized access
5. **Use Branch Protection**: Require reviews before merging to main

## Additional Resources

- [DockerHub Documentation](https://docs.docker.com/docker-hub/)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Docker Buildx Documentation](https://docs.docker.com/buildx/working-with-buildx/)
- [GitHub Secrets Documentation](https://docs.github.com/en/actions/security-guides/encrypted-secrets)

## Questions?

If you encounter issues:
1. Check the GitHub Actions logs for detailed error messages
2. Review this guide for common solutions
3. Open an issue in the repository
4. Consult DockerHub and GitHub Actions documentation
