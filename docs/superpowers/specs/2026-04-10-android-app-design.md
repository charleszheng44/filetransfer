# Android app for `ftr` — design

**Date:** 2026-04-10
**Issue:** [charleszheng44/filetransfer#1](https://github.com/charleszheng44/filetransfer/issues/1)
**Status:** Draft (pending user review)

## Goal

Ship a first-class Android client for `ftr` that is fully interoperable with the existing Go CLI (`main.go`) and iOS app (`ios-app/`). The Android app lets a user:

- **Send** any file or folder from their Android device to any `ftr` peer on the LAN (Go CLI, iOS, or another Android device).
- **Receive** files and folders from any `ftr` peer, with the same passkey-based auth and drop-dir model as the other clients.

No changes to the Go CLI or iOS app. The Android app conforms to the existing wire protocol.

## Non-goals (v1)

- Background / foreground-service receiver. v1 receiver runs only while the app is in the foreground, matching iOS behavior. Foreground-service mode is an opt-in v2 addition.
- Bluetooth or Wi-Fi Direct fallback.
- Public `Downloads/` via MediaStore, or SAF-picked drop directory.
- Multi-file select, resumable transfers, byte-level progress bar.
- UI tests, instrumentation tests, CI wiring.

## Tech stack

| Decision | Choice | Why |
|---|---|---|
| Language / UI | Kotlin + Jetpack Compose | Modern analogue to SwiftUI; clean mirror of existing iOS screens. |
| Min SDK | 26 (Android 8.0) | ~97% device coverage; no legacy storage workarounds. |
| Target SDK | Latest stable at implementation time | Standard Android app practice. |
| mDNS | `android.net.nsd.NsdManager` | Built-in; supports TXT records from API 21+. |
| HTTP server | Ktor (CIO engine) | Coroutine-native, idiomatic, handles multipart cleanly. |
| HTTP client | Ktor client (CIO) | Shares engine/config with the server. |
| Tar + gzip | Apache Commons Compress | Robust vs. hand-rolling; ~700 KB APK cost. |
| Received-file storage | `context.getExternalFilesDir(null)/drop/` | No runtime permissions; real `File` paths for tar extraction; visible in Android Files app via `FileProvider`. |
| Send-file picking | SAF `ActivityResultContracts.OpenDocument` / `OpenDocumentTree` | Lets the user pick any file or folder on the device. |
| Receiver lifecycle | Activity-scoped (v1) | Mirrors iOS; foreground-service deferred to v2. |

## Project layout

Gradle project at `android-app/` (peer of `ios-app/`). Single app module, Kotlin DSL.

```
android-app/
├── build.gradle.kts                (top-level)
├── settings.gradle.kts
├── gradle.properties
├── gradle/libs.versions.toml       (version catalog)
└── app/
    ├── build.gradle.kts
    ├── proguard-rules.pro
    └── src/
        ├── main/
        │   ├── AndroidManifest.xml
        │   ├── java/com/filetransfer/ftr/
        │   │   ├── FtrApplication.kt
        │   │   ├── MainActivity.kt
        │   │   ├── model/Peer.kt                  ← mirrors Peer.swift
        │   │   ├── net/
        │   │   │   ├── NsdAdvertiser.kt           ← mirrors BonjourAdvertiser.swift
        │   │   │   ├── NsdBrowser.kt              ← mirrors BonjourBrowser.swift
        │   │   │   ├── FileReceiver.kt            ← mirrors FileReceiver.swift
        │   │   │   └── FileSender.kt              ← mirrors FileSender.swift
        │   │   ├── archive/TarGz.kt               ← Commons Compress wrapper
        │   │   ├── ui/
        │   │   │   ├── theme/{Theme,Color,Type}.kt
        │   │   │   ├── MainScreen.kt              ← mirrors ContentView.swift
        │   │   │   ├── send/
        │   │   │   │   ├── PeerListScreen.kt      ← mirrors PeerListView.swift
        │   │   │   │   └── PeerListViewModel.kt
        │   │   │   └── receive/
        │   │   │       ├── ReceiverScreen.kt      ← mirrors ReceiverView.swift
        │   │   │       └── ReceiverViewModel.kt
        │   │   └── util/PassKey.kt
        │   └── res/
        │       ├── values/strings.xml
        │       ├── xml/file_paths.xml             ← FileProvider paths
        │       └── xml/network_security_config.xml
        └── test/
            ├── java/com/filetransfer/ftr/
            │   ├── archive/TarGzTest.kt
            │   └── net/
            │       ├── FileSenderHttpTest.kt
            │       └── NsdAdvertiserTxtRecordTest.kt
            └── resources/
                └── fixtures/
                    └── go-zeroconf-txt.bin         ← captured Go TXT bytes
```

**Rules:**
- `net/` owns everything that talks the network. Each class is constructed with its dependencies (drop dir, passkey, etc.) and can be unit-tested in isolation.
- `archive/TarGz.kt` is a pure-function object. No Android dependencies. Trivially testable on JVM.
- `ui/` is split by feature (`send/`, `receive/`). Each screen + its ViewModel live together.
- `FtrApplication` holds singletons (`NsdBrowser`, `FileReceiver`) so they survive config changes and both tabs share them.
- Dependency direction: `ui → net → archive`. No back-edges.

**Core dependencies** (pinned in the version catalog at implementation time):

- `androidx.compose.*` (BOM)
- `androidx.lifecycle:lifecycle-viewmodel-compose`
- `io.ktor:ktor-server-core`, `ktor-server-cio`
- `io.ktor:ktor-client-core`, `ktor-client-cio`
- `org.apache.commons:commons-compress`
- `org.jetbrains.kotlinx:kotlinx-coroutines-android`

**Manifest essentials:**

- `<uses-permission android:name="android.permission.INTERNET" />`
- `<uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />`
- `<uses-permission android:name="android.permission.CHANGE_WIFI_MULTICAST_STATE" />` (needed to hold a `MulticastLock` while browsing NSD on some devices).
- `<provider>` for `androidx.core.content.FileProvider` pointing at `res/xml/file_paths.xml`, exposing the drop dir to Android's Files app.
- `networkSecurityConfig` allowing cleartext HTTP on private network addresses (the Go CLI uses plain HTTP).

## Wire protocol contract

The Android app MUST match the existing protocol bit-for-bit. Taken from `main.go` and `FileReceiver.swift`.

**Service advertisement (mDNS):**

- Service type: `_ftr._tcp`
- Domain: `local.`
- Service name: user-supplied device name. Default = `android.os.Build.MODEL` with leading/trailing whitespace trimmed and any embedded `.` replaced with `-` (the first `.` in a Bonjour service instance name acts as a delimiter on some clients). Mirrors `os.Hostname()` on Go and `UIDevice.current.name` on iOS.
- Port: user-supplied, default `8844` (`defaultPort` at `main.go:28`).
- TXT record: single attribute where the key is the drop-dir path and the value is empty. Matches Go's `zeroconf.Register(..., []string{dropDir}, nil)` at `main.go:137` and iOS `NWTXTRecord([dropDir: ""])` at `FileReceiver.swift:29`.

**TXT record encoding quirk:** Android's `NsdServiceInfo.setAttribute(key, null)` serializes a key with no `=value` suffix; `setAttribute(key, "")` serializes `key=`. `NsdAdvertiserTxtRecordTest` pins the correct form against a Go-produced fixture (`fixtures/go-zeroconf-txt.bin`). Whichever matches, codify it in `NsdAdvertiser`.

**Transfer endpoint:**

- `POST /upload` (hard-coded in `main.go:496`).
- Body: `multipart/form-data` with one part named `file`.
- Required headers:
  - `X-Ftr-Passkey: <passkey>` (constant `passKeyHeader` at `main.go:34`) — reject with 401 if mismatched.
  - `X-Ftr-File-Type: file` or `dir` (constant `fileTypeHeader` at `main.go:35`).
  - `Content-Type: multipart/form-data; boundary=<boundary>`.
- Multipart filename for a single file is `basename(src)`.
- For a directory, the sender tar+gzips into `<dirname>.tar.gz`, sets `X-Ftr-File-Type: dir`, and the receiver untars + deletes the tarball (`main.go:371`).

**Response codes:**

- `200 OK` on success.
- `400 Bad Request` for malformed multipart, missing `Content-Length`, etc.
- `401 Unauthorized` on passkey mismatch.
- `405 Method Not Allowed` on non-POST.
- `409 Conflict` if the destination file already exists (`main.go:351`).
- `500 Internal Server Error` on server-side failure.

**Conflict semantics:** the receiver refuses to overwrite existing files. The Android receiver does the same — bail with 409 before opening the output file.

**Tar format specifics** (for Commons Compress configuration):

- POSIX ustar with 512-byte blocks (Go default).
- Directory entries have a trailing `/` and typeflag `5`.
- Symlinks are **skipped** on the sender side (not followed, not archived) — `main.go:227`.
- The top-level directory is **not** emitted as an entry; only its contents, with relative paths — `main.go:214`. Commons Compress needs the entry name to be the *relative* path, not the absolute one.
- Directory permission fix-up: if any of owner/group/other read bits are set, the corresponding execute bit is set too (`main.go:236-244`). The Android sender must replicate this so the receiver can enter the directory after extraction.
- Gzip wrapper around the tar stream uses defaults; `GzipCompressorOutputStream` from Commons Compress matches Go's `compress/gzip` defaults.

## Data flows

### Send flow (single file)

1. User opens the Send tab. `PeerListViewModel` exposes `peers: StateFlow<List<Peer>>` backed by `NsdBrowser`.
2. `NsdBrowser.start()`:
   - Acquires a `WifiManager.MulticastLock`.
   - Calls `NsdManager.discoverServices("_ftr._tcp", PROTOCOL_DNS_SD, listener)`.
   - For each `onServiceFound`, calls `NsdManager.resolveService` to populate host, port, and TXT attributes, then emits a `Peer`.
3. User taps a peer. A dialog prompts for the passkey; on confirm, the UI launches `rememberLauncherForActivityResult(OpenDocument())`.
4. SAF returns a `content://` URI. `FileSender.sendFile(uri, peer, passKey)` runs in `viewModelScope`:
   - Resolves the display name via `ContentResolver.query(uri, [DISPLAY_NAME], ...)`. This becomes the multipart `filename=`.
   - Opens a stream via `contentResolver.openInputStream(uri)`.
   - Builds a Ktor `MultiPartFormDataContent` with a single `PartData.BinaryItem` named `file`.
   - Sets `X-Ftr-Passkey` and `X-Ftr-File-Type: file`.
   - `POST http://${peer.host}:${peer.port}/upload`.
   - Updates `sendState: StateFlow<SendState>` (`Idle | Sending | Success(msg) | Error(msg)`).

### Send flow (directory)

1. Same through step 3, but the user picks "Send Folder" which uses `OpenDocumentTree()`.
2. `FileSender.sendDirectory(treeUri, peer, passKey)`:
   - Walks the tree recursively via `DocumentFile.fromTreeUri(...).listFiles()`.
   - Streams each entry through `TarGz.compress(entries, outputFile)` into a temp `.tar.gz` under `context.cacheDir`.
   - Sends the tarball with `X-Ftr-File-Type: dir` and multipart filename `<dirname>.tar.gz` (matches `main.go:186`).
   - Deletes the cache tarball in a `finally` block.

### Receive flow

1. User opens the Receive tab, sets name/port/passkey, taps "Join Network".
2. `ReceiverViewModel.start()` calls `FileReceiver.start(port, name, passKey)`:
   - Ensures `drop/` exists under `getExternalFilesDir(null)`.
   - Starts a Ktor CIO server bound to `0.0.0.0:<port>` with a single route:
     ```kotlin
     post("/upload") { handleUpload(call, dropDir, passKey) }
     ```
   - Starts `NsdAdvertiser.register(name, port, dropDir)` via `NsdManager.registerService`.
3. `handleUpload`:
   - Auth: if `X-Ftr-Passkey` mismatches, respond 401 and return.
   - Iterate `call.receiveMultipart()`, grab the first `PartData.FileItem` named `file`.
   - Sanitize the filename (strip path separators, reject `.` and `..`).
   - Check `File(dropDir, fileName).exists()` → 409 if yes.
   - Stream `PartData.FileItem.provider()` into the target `File`.
   - If `X-Ftr-File-Type == "dir"`: call `TarGz.decompress(tarball, dropDir)`, delete the tarball.
   - Emit the new file (or directory) name into `receivedFiles: StateFlow<List<String>>`.
   - Respond 200.
4. "Leave Network" cancels the Ktor server, unregisters NSD, releases the multicast lock.

## Error handling

### Send side

| Failure | Behavior |
|---|---|
| No peers found | Empty-state view (mirrors `PeerListView.swift:22`). No error banner. |
| Peer resolve fails | Drop the peer from the list silently, log at `Log.w`. |
| Passkey rejected (401) | `SendState.Error("Invalid passkey")` in the status bar. |
| File already exists on receiver (409) | `SendState.Error("File already exists on <peer>")`. No auto-rename. |
| Network unreachable / connection refused | `SendState.Error("Could not reach <peer>: <reason>")`. |
| User cancels SAF picker | No-op. |
| Tar compression fails mid-stream | Delete the partial cache tarball in `finally`, show `"Failed to package folder: <reason>"`. |

### Receive side

| Failure | Behavior |
|---|---|
| Port already in use (`BindException`) | `ReceiverState.Error("Port $port is already in use")`, receiver stays stopped. |
| NSD registration fails | Log + `Error("Failed to advertise on network")`; tear down the Ktor server too so we never listen without being discoverable. |
| Malformed multipart body | Respond 400, log warning, do not crash the coroutine. |
| Disk full during write | Respond 500, delete the partially-written file. |
| Tar extraction fails | Respond 500, delete the tarball and any partially extracted contents under `drop/<name>/`. |
| Zip-slip attempt (entry name like `../../etc/passwd`) | Reject the entry, respond 500. |

### Three cross-cutting concerns

1. **Zip-slip guard is mandatory.** Before writing any tar entry, compute `File(dropDir, entry.name).canonicalPath` and verify it starts with `dropDir.canonicalPath + File.separator`. Commons Compress does **not** do this for you. The Go and iOS sides lack this guard today — a follow-up issue should add it there once the Android work lands.
2. **Coroutine cancellation on teardown.** The Ktor server and NSD registration live under a single `SupervisorJob()` tied to `FileReceiver`'s lifecycle, so "Leave Network" cancels everything deterministically. No leaked listener threads.
3. **Multicast lock.** `NsdBrowser.stop()` and `NsdAdvertiser.stop()` release the `MulticastLock` in their `finally` blocks. Holding one after teardown drains battery and is an easy-to-miss regression.

## Testing strategy

Intentionally minimal. This is a small utility app, not a platform.

| Test | Layer | Rationale |
|---|---|---|
| `TarGzTest.roundTrip` | Unit (JVM) | Compress a tree → decompress → assert file contents, permissions, and directory structure match. Protects against Commons Compress config drift. |
| `TarGzTest.goFixture` | Unit (JVM) | Decompress a `.tar.gz` fixture produced by the Go CLI (checked in under `app/src/test/resources/`). Assert contents. This is the one test that catches protocol drift between the three clients. |
| `TarGzTest.zipSlip` | Unit (JVM) | Feed a malicious tarball with `../` entries, assert extraction refuses. |
| `NsdAdvertiserTxtRecordTest` | Unit (JVM) | Decode `fixtures/go-zeroconf-txt.bin` (captured Go TXT bytes for a known drop-dir path) and assert the advertiser's encoded TXT record matches. Catches the `null` vs `""` attribute encoding mismatch. |
| `FileSenderHttpTest` | Unit (JVM) | Stub Ktor `MockEngine`, assert headers and multipart body shape match the Go receiver. |
| Manual interop matrix | Integration | Documented in the PR. Combinations: Android↔Go, Android↔iOS, Android↔Android, for a single file and a nested directory. |

No instrumentation tests, UI tests, or CI wiring for v1. A GitHub Actions `./gradlew test` job is trivial to add as a follow-up if desired.

## Installation & testing (for the human operator)

### Android — installing the debug build

Three paths depending on how you work:

1. **Android Studio.** Open `android-app/`, plug in a phone with Developer Options → USB debugging enabled (or launch an AVD), click Run.
2. **Command line.**
   ```
   cd android-app
   ./gradlew installDebug
   adb shell am start -n com.filetransfer.ftr/.MainActivity
   ```
   Requires `adb` from Android SDK platform-tools on `$PATH`; `adb devices` must list the device.
3. **Sideload an APK.**
   ```
   ./gradlew assembleDebug
   adb install -r app/build/outputs/apk/debug/app-debug.apk
   ```
   Or copy the APK to the phone and tap it in the Files app with "Install unknown apps" enabled for that file manager.

**Network gotcha:** the phone and the peer must be on the same Wi-Fi network, and the network must allow multicast/mDNS traffic. Most home Wi-Fi does; many corporate and campus networks block mDNS between client-isolated devices, which makes peer discovery silently fail. If discovery breaks on a new network, test from a laptop on the same Wi-Fi with `ftr list` to confirm whether the network or the app is at fault.

### iOS — installing on a device

1. **Free Apple ID sideload** (7-day signing).
   - Open `ios-app/FileTransfer.xcodeproj`.
   - Xcode → Settings → Accounts → add your Apple ID.
   - Select the `FileTransfer` target → Signing & Capabilities → pick your personal team.
   - Change the bundle identifier to something unique (e.g., `com.<yourname>.FileTransfer`).
   - Plug in an iPhone, select it as the run destination, hit Run.
   - First launch: on the phone, Settings → General → VPN & Device Management → trust your developer profile.
   - The app stops working after 7 days and needs re-installing from Xcode.
2. **Paid Apple Developer Program** ($99/yr).
   - Same flow but pick a paid team in Signing. App lasts a year between re-signs. TestFlight available for remote testers.
3. **Simulator only.** Pick any iOS simulator as the run destination and Run. The iOS simulator, an Android emulator, and the Go CLI can all run on the same Mac and will discover each other over the host network.

**Permission prompts on first launch:**

- **iOS:** "FileTransfer would like to find and connect to devices on your local network." → allow. Handled by `NSLocalNetworkUsageDescription` in `Info.plist:5`.
- **Android:** no runtime permission prompt. `INTERNET` and `CHANGE_WIFI_MULTICAST_STATE` are install-time permissions, and no dangerous permissions are used since storage is app-private.

## Open questions

- [ ] Confirm whether Go's `zeroconf` parses Android's `NsdServiceInfo.setAttribute(path, null)` TXT encoding correctly, or whether `setAttribute(path, "")` is required instead. Decide with a byte-level fixture test before declaring v1 done.
- [ ] Verify whether `CHANGE_WIFI_MULTICAST_STATE` is actually required on API 26+ — some docs say it is only needed on older versions. If not, drop it from the manifest.
