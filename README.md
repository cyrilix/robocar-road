# robocar-road

Process camera frames to detect road contours

## Docker

To build images, run:
```bash 
docker buildx build . --platform linux/arm/7,linux/arm64,linux/amd64 -t cyrilix/robocar-road
```
