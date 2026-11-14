# Building

## Desktop (development)

```
cd apps/lemondrop-mobile
go test ./...
go run ./cmd/lemondrop-mobile                     # phone-shaped window, mouse = finger
go run ./cmd/lemondrop-mobile -inset-top 59 -inset-bottom 34   # simulate a notch
go run ./cmd/lemondrop-mobile -start drop -speed 2 -frames 900 -shot drop.png -mute
```

Keyboard: ←/→ move, ↑/X turn, Z turn back, ↓ soft drop, Space drop, P/Esc pause.

## iOS — verified: simulator build, launch and play (iPhone 17, iOS 27)

Requirements: Xcode, Go, XcodeGen (`brew install xcodegen`).

```
scripts/build-ios.sh             # ebitenmobile bind → Mobile.xcframework, xcodegen, simulator build
xcrun simctl install booted ios/build/dd/Build/Products/Debug-iphonesimulator/LemonDrop.app
xcrun simctl launch booted com.limoni.lemondrop
SIMCTL_CHILD_LEMONDROP_START=drop xcrun simctl launch booted com.limoni.lemondrop   # open a screen directly
```

Physical device:
1. Open `ios/LemonDrop.xcodeproj`. Select the LemonDrop target, then Signing &
   Capabilities. Pick your Team, or set `DEVELOPMENT_TEAM` in
   `ios/project.yml` and run `xcodegen generate` again.
2. Change `PRODUCT_BUNDLE_IDENTIFIER` if `com.limoni.lemondrop` is taken.
3. Select the device and run. For the App Store: Product › Archive.

Notes:
- The app adopts the UIScene lifecycle (`SceneDelegate`). iOS 27 terminates
  apps that do not.
- Save data is in Application Support.
- Haptics use `UIImpactFeedbackGenerator` and `UINotificationFeedbackGenerator`.
- The name prompt is a `UIAlertController`.

## Android — not built here (no Android SDK on the build machine)

Requirements: Go, JDK 17, Android SDK (platform 35), Android NDK,
`ANDROID_HOME`, and Gradle 8.9+ (or create the wrapper with `gradle wrapper`).

```
export ANDROID_HOME=~/Library/Android/sdk
scripts/build-android.sh         # ebitenmobile bind → app/libs/lemondrop.aar, then assembleDebug
adb install android/app/build/outputs/apk/debug/app-debug.apk
```

Release signing reads a keystore that is kept outside the repository:

```
keytool -genkeypair -v -keystore lemondrop.jks -alias lemondrop -keyalg RSA -keysize 2048 -validity 10000
export LEMONDROP_KEYSTORE=$PWD/lemondrop.jks LEMONDROP_KEYSTORE_PASSWORD=… LEMONDROP_KEY_ALIAS=lemondrop LEMONDROP_KEY_PASSWORD=…
cd android && gradle assembleRelease      # or bundleRelease for Play
```

The shell (`MainActivity.kt`):
- draws edge to edge and passes system-bar and cutout insets to the game;
- suspends and resumes the game with the activity;
- maps haptics to `performHapticFeedback`, which needs no permission;
- asks for the player's name with an `AlertDialog`.

## Scores
Runs are kept on the device (top 20). They are not sent to the terminal
game's shared `/drop/scores` board, because the Director deals differently
from the classic bag. A future online board for this game would need its own
endpoint in `apps/scoreboard`; the run log (`core.Sim.ClearLog`, `DropPts`,
`Pieces`) already holds what such a server needs to recompute the score.

## Save data and migration
- `lemondrop-mobile.json` in the data directory, `"version": 1`. Files from a
  newer version are not loaded.
- On first start on desktop, the terminal game's `limoni/lemondrop.json` (name
  and board, same scoring) is imported into the local board once.
