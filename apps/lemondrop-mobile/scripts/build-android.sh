#!/bin/sh
# Builds the Go game as an Android library and the debug APK.
# Needs: Go, JDK 17, Android SDK (platform 35) and NDK, ANDROID_HOME set.
set -eu
cd "$(dirname "$0")/.."
: "${ANDROID_HOME:?set ANDROID_HOME to your Android SDK}"
go install github.com/hajimehoshi/ebiten/v2/cmd/ebitenmobile@v2.10.6
"$(go env GOPATH)/bin/ebitenmobile" bind -target android -androidapi 24 \
  -javapkg com.limoni.lemondrop -o android/app/libs/lemondrop.aar ./mobile
cd android
if [ -x ./gradlew ]; then G=./gradlew; else G=gradle; fi
$G assembleDebug
echo "APK: android/app/build/outputs/apk/debug/app-debug.apk"
