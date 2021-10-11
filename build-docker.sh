#! /bin/bash

IMAGE_NAME=robocar-road
BINARY_NAME=rc-road

TAG=$(git describe)
FULL_IMAGE_NAME=docker.io/cyrilix/${IMAGE_NAME}:${TAG}
OPENCV_VERSION=4.5.3
SRC_CMD=./cmd/$BINARY_NAME
GOLANG_VERSION=1.17

image_build(){
  local containerName=builder

  GOPATH=/go

  buildah from --name ${containerName} docker.io/cyrilix/opencv-buildstage:${OPENCV_VERSION}
  buildah config --label maintainer="Cyrille Nofficial" "${containerName}"

  buildah copy --from=docker.io/library/golang:${GOLANG_VERSION} "${containerName}" /usr/local/go /usr/local/go
  buildah config --env GOPATH=/go \
                 --env PATH=/usr/local/go/bin:$GOPATH/bin:/usr/local/go/bin:/usr/bin:/bin \
                 "${containerName}"

  buildah run \
    --env GOPATH=${GOPATH} \
    "${containerName}" \
    mkdir -p /src "$GOPATH/src" "$GOPATH/bin"

  buildah run \
    --env GOPATH=${GOPATH} \
    "${containerName}" \
    chmod -R 777 "$GOPATH"


  buildah config --env PKG_CONFIG_PATH=/usr/local/lib/pkgconfig:/usr/local/lib64/pkgconfig "${containerName}"
  buildah config --workingdir /src/ "${containerName}"

  buildah add "${containerName}" . .

  for platform in "linux/amd64" "linux/arm64" "linux/arm/v7"
  do

    GOOS=$(echo "$platform" | cut -f1 -d/) && \
    GOARCH=$(echo "$platform" | cut -f2 -d/) && \
    GOARM=$(echo "$platform" | cut -f3 -d/ | sed "s/v//" )

    case $GOARCH in
      "amd64")
        ARCH=amd64
        ARCH_LIB_DIR=/usr/lib/x86_64-linux-gnu
        EXTRA_LIBS=""
        CC=gcc
        CXX=g++
      ;;
      "arm64")
        ARCH=arm64
        ARCH_LIB_DIR=/usr/lib/aarch64-linux-gnu
        EXTRA_LIBS="-ltbb"
        CC=aarch64-linux-gnu-gcc
        CXX=aarch64-linux-gnu-g++
      ;;
      "arm")
        ARCH=armhf
        ARCH_LIB_DIR=/usr/lib/arm-linux-gnueabihf
        EXTRA_LIBS="-ltbb"
        CC=arm-linux-gnueabihf-gcc
        CXX=arm-linux-gnueabihf-g++
      ;;
    esac

    printf "Build binary for %s\n\n" "${platform}"

    buildah run \
      --env CGO_ENABLED=1 \
      --env CC=${CC} \
      --env CXX=${CXX} \
      --env GOOS=${GOOS} \
      --env GOARCH=${GOARCH} \
      --env GOARM=${GOARM} \
      --env CGO_CPPFLAGS="-I/opt/opencv/${ARCH}/include/opencv4/" \
      --env CGO_LDFLAGS="-L/opt/opencv/${ARCH}/lib -L${ARCH_LIB_DIR} ${EXTRA_LIBS} -lopencv_core -lopencv_face -lopencv_videoio -lopencv_imgproc -lopencv_highgui -lopencv_imgcodecs -lopencv_objdetect -lopencv_features2d -lopencv_video -lopencv_dnn -lopencv_xfeatures2d -lopencv_calib3d -lopencv_photo -lopencv_flann" \
      --env CGO_CXXFLAGS="--std=c++1z" \
      "${containerName}" \
      go build  -tags customenv -a -o ${BINARY_NAME}.${ARCH} ${SRC_CMD}

  done
  buildah commit --rm ${containerName} ${IMAGE_NAME}-builder
}

image_final(){
  local containerName=runtime

  for platform in "linux/amd64" "linux/arm64" "linux/arm/v7"
  do

    GOOS=$(echo $platform | cut -f1 -d/) && \
    GOARCH=$(echo $platform | cut -f2 -d/) && \
    GOARM=$(echo $platform | cut -f3 -d/ | sed "s/v//" )
    VARIANT="--variant $(echo $platform | cut -f3 -d/  )"

    if [[ -z "$GOARM" ]] ;
    then
      VARIANT=""
    fi

    if [[ "${GOARCH}" == "arm" ]]
    then
      BINARY="${BINARY_NAME}.armhf"
    else
      BINARY="${BINARY_NAME}.${GOARCH}"
    fi

    buildah from --name "${containerName}" --os "${GOOS}" --arch "${GOARCH}" ${VARIANT} docker.io/cyrilix/opencv-runtime:${OPENCV_VERSION}

    buildah copy --from ${IMAGE_NAME}-builder  "$containerName" "/src/${BINARY}" /usr/local/bin/${BINARY_NAME}

    buildah config --label maintainer="Cyrille Nofficial" "${containerName}"
    buildah config --user 1234 "$containerName"
    buildah config --cmd '' "$containerName"
    buildah config --entrypoint '[ "/usr/local/bin/'${BINARY_NAME}'" ]' "$containerName"

    buildah commit --rm --manifest ${IMAGE_NAME} ${containerName}
  done
}
#buildah rmi localhost/$IMAGE_NAME
#buildah manifest rm localhost/${IMAGE_NAME}

image_build linux/amd64
image_build linux/arm64
image_build linux/arm/v7

image_build

# push image
image_final
printf "\n\nPush manifest to %s\n\n" ${FULL_IMAGE_NAME}
buildah manifest push --rm -f v2s2 "localhost/$IMAGE_NAME" "docker://$FULL_IMAGE_NAME" --all