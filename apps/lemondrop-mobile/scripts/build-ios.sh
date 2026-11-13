#!/bin/sh
# Builds the Go game as an iOS framework, generates the Xcode project and
# builds for the simulator. Needs: Go, Xcode, XcodeGen (brew install xcodegen).
set -eu
cd "$(dirname "$0")/.."
go install github.com/hajimehoshi/ebiten/v2/cmd/ebitenmobile@v2.10.6
"$(go env GOPATH)/bin/ebitenmobile" bind -target ios -o ios/build/Mobile.xcframework ./mobile
cd ios
xcodegen generate
xcodebuild -project LemonDrop.xcodeproj -scheme LemonDrop -sdk iphonesimulator \
  -destination "${DEST:-platform=iOS Simulator,name=iPhone 17}" \
  -derivedDataPath build/dd CODE_SIGNING_ALLOWED=NO build
echo "App: ios/build/dd/Build/Products/Debug-iphonesimulator/LemonDrop.app"
