#!/bin/bash
# QTRP 跨平台构建脚本

VERSION=${VERSION:-"1.0.0"}
BUILD_DIR="dist"
APP_NAME="qtrp"

# 清理
rm -rf ${BUILD_DIR}
mkdir -p ${BUILD_DIR}

# 构建目标平台
PLATFORMS=(
    "linux/amd64"
#    "linux/arm64"
#    "windows/amd64"
)

for PLATFORM in "${PLATFORMS[@]}"; do
    GOOS=${PLATFORM%/*}
    GOARCH=${PLATFORM#*/}
    
    OUTPUT_DIR="${BUILD_DIR}/${APP_NAME}-${VERSION}-${GOOS}-${GOARCH}"
    mkdir -p ${OUTPUT_DIR}
    
    # 可执行文件后缀
    EXT=""
    if [ "$GOOS" = "windows" ]; then
        EXT=".exe"
    fi
    
    echo "Building for ${GOOS}/${GOARCH}..."
    
    # 构建服务端
    GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="-s -w" \
        -o "${OUTPUT_DIR}/${APP_NAME}-server${EXT}" ./cmd/server
    
    # 构建客户端
    GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="-s -w" \
        -o "${OUTPUT_DIR}/${APP_NAME}-client${EXT}" ./cmd/client
    
    # 复制配置文件
    cp -r configs ${OUTPUT_DIR}/
    
#    # 打包
#    cd ${BUILD_DIR}
#    if [ "$GOOS" = "windows" ]; then
#        zip -r "${APP_NAME}-${VERSION}-${GOOS}-${GOARCH}.zip" "${APP_NAME}-${VERSION}-${GOOS}-${GOARCH}"
#    else
#        tar -czvf "${APP_NAME}-${VERSION}-${GOOS}-${GOARCH}.tar.gz" "${APP_NAME}-${VERSION}-${GOOS}-${GOARCH}"
#    fi
#    cd ..
    
    echo "Done: ${OUTPUT_DIR}"
done

echo ""
echo "Build complete! Output:"
ls -la ${BUILD_DIR}/*.{tar.gz,zip} 2>/dev/null
