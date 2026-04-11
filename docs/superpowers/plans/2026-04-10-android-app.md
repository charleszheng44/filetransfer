# Android app for `ftr` — implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a Kotlin/Compose Android client at `android-app/` that speaks the existing `ftr` wire protocol bit-for-bit, so an Android phone can send and receive files and folders to/from the Go CLI and iOS app on the same LAN.

**Architecture:** Single Gradle module. `ui → net → archive` one-way dependency. `FileReceiver` runs an embedded Ktor CIO server under a coroutine supervisor job; `FileSender` uses the Ktor client. `NsdAdvertiser`/`NsdBrowser` wrap `NsdManager`. `TarGz` wraps Apache Commons Compress. Received files land in `context.getExternalFilesDir(null)/drop/`. Receiver is activity-scoped (v1), mirroring iOS behavior.

**Tech Stack:** Kotlin, Jetpack Compose (Material3), Ktor 2.x (CIO server + client), Apache Commons Compress, AndroidX Lifecycle, kotlinx-coroutines, minSdk 26.

**Spec:** `docs/superpowers/specs/2026-04-10-android-app-design.md`
**Issue:** charleszheng44/filetransfer#1

---

## File structure

Files created by this plan, grouped by the task that creates them:

```
android-app/
├── .gitignore                                        [Task 1]
├── settings.gradle.kts                               [Task 1]
├── build.gradle.kts                                  [Task 1]
├── gradle.properties                                 [Task 1]
├── gradle/libs.versions.toml                         [Task 1]
├── gradle/wrapper/gradle-wrapper.properties          [Task 1]
├── gradlew, gradlew.bat, gradle/wrapper/*.jar        [Task 1]
└── app/
    ├── build.gradle.kts                              [Task 1]
    ├── proguard-rules.pro                            [Task 1]
    └── src/
        ├── main/
        │   ├── AndroidManifest.xml                   [Task 1, extended Task 12]
        │   ├── java/com/filetransfer/ftr/
        │   │   ├── FtrApplication.kt                 [Task 9]
        │   │   ├── MainActivity.kt                   [Task 1, finalized Task 9]
        │   │   ├── model/Peer.kt                     [Task 2]
        │   │   ├── util/PassKey.kt                   [Task 2]
        │   │   ├── archive/TarGz.kt                  [Task 3]
        │   │   ├── net/
        │   │   │   ├── NsdAdvertiser.kt              [Task 4]
        │   │   │   ├── NsdBrowser.kt                 [Task 5]
        │   │   │   ├── FileReceiver.kt               [Task 6]
        │   │   │   └── FileSender.kt                 [Tasks 7, 8]
        │   │   └── ui/
        │   │       ├── theme/{Theme,Color,Type}.kt   [Task 9]
        │   │       ├── MainScreen.kt                 [Task 9]
        │   │       ├── send/
        │   │       │   ├── PeerListScreen.kt         [Task 11]
        │   │       │   └── PeerListViewModel.kt      [Task 11]
        │   │       └── receive/
        │   │           ├── ReceiverScreen.kt         [Task 10]
        │   │           └── ReceiverViewModel.kt      [Task 10]
        │   └── res/
        │       ├── values/strings.xml                [Task 1]
        │       ├── values/themes.xml                 [Task 1]
        │       ├── xml/file_paths.xml                [Task 12]
        │       └── xml/network_security_config.xml   [Task 12]
        └── test/
            ├── java/com/filetransfer/ftr/
            │   ├── archive/TarGzTest.kt              [Task 3]
            │   └── net/
            │       ├── NsdAdvertiserTxtRecordTest.kt [Task 4]
            │       └── FileSenderHttpTest.kt         [Task 7]
            └── resources/fixtures/
                └── go-zeroconf-txt.bin               [Task 4]
```

Modified at the end (Task 13): `README.md` gains an "Android" section and a manual interop-test checklist.

---

## Prerequisites

Before Task 1, the executor needs:

- JDK 17+ installed and on `$PATH` (`java -version` prints 17 or higher).
- Android SDK installed, with `platform-tools/` and `cmdline-tools/latest/bin/` on `$PATH`. Verify with `sdkmanager --version`.
- `ANDROID_HOME` env var pointing at the SDK root. On Linux this is typically `~/Android/Sdk`.
- An Android device with USB debugging, or an AVD emulator running API 26+.
- Run `sdkmanager --install "platforms;android-34" "build-tools;34.0.0"` if those are missing.

These are environmental prereqs, not part of any task.

---

## Task 1: Scaffold the Gradle project and a running "Hello" activity

**Goal:** Produce a buildable, runnable Android app with correct Kotlin DSL Gradle setup, version catalog, minSdk 26, Compose enabled, and an empty `MainActivity` that displays "FileTransfer" on screen. No networking yet — just proving the build works end-to-end.

**Files:**
- Create: `android-app/.gitignore`
- Create: `android-app/settings.gradle.kts`
- Create: `android-app/build.gradle.kts`
- Create: `android-app/gradle.properties`
- Create: `android-app/gradle/libs.versions.toml`
- Create: `android-app/gradle/wrapper/gradle-wrapper.properties`
- Create: `android-app/gradlew`, `android-app/gradlew.bat`, `android-app/gradle/wrapper/gradle-wrapper.jar` (via `gradle wrapper`)
- Create: `android-app/app/build.gradle.kts`
- Create: `android-app/app/proguard-rules.pro`
- Create: `android-app/app/src/main/AndroidManifest.xml`
- Create: `android-app/app/src/main/res/values/strings.xml`
- Create: `android-app/app/src/main/res/values/themes.xml`
- Create: `android-app/app/src/main/java/com/filetransfer/ftr/MainActivity.kt`

- [ ] **Step 1.1: Create `android-app/.gitignore`**

```gitignore
# Gradle
.gradle/
build/
!gradle/wrapper/gradle-wrapper.jar
!gradle-wrapper.properties

# IntelliJ / Android Studio
.idea/
*.iml
local.properties
captures/
.externalNativeBuild/
.cxx/

# macOS
.DS_Store
```

- [ ] **Step 1.2: Create `android-app/gradle.properties`**

```properties
org.gradle.jvmargs=-Xmx2048m -Dfile.encoding=UTF-8
org.gradle.parallel=true
org.gradle.caching=true
android.useAndroidX=true
android.nonTransitiveRClass=true
kotlin.code.style=official
```

- [ ] **Step 1.3: Create `android-app/gradle/libs.versions.toml` (version catalog)**

```toml
[versions]
agp = "8.5.2"
kotlin = "2.0.20"
coreKtx = "1.13.1"
lifecycle = "2.8.5"
activityCompose = "1.9.2"
composeBom = "2024.09.02"
coroutines = "1.8.1"
ktor = "2.3.12"
commonsCompress = "1.27.1"
junit = "4.13.2"

[libraries]
androidx-core-ktx = { group = "androidx.core", name = "core-ktx", version.ref = "coreKtx" }
androidx-lifecycle-runtime-ktx = { group = "androidx.lifecycle", name = "lifecycle-runtime-ktx", version.ref = "lifecycle" }
androidx-lifecycle-viewmodel-compose = { group = "androidx.lifecycle", name = "lifecycle-viewmodel-compose", version.ref = "lifecycle" }
androidx-activity-compose = { group = "androidx.activity", name = "activity-compose", version.ref = "activityCompose" }
androidx-compose-bom = { group = "androidx.compose", name = "compose-bom", version.ref = "composeBom" }
androidx-compose-ui = { group = "androidx.compose.ui", name = "ui" }
androidx-compose-ui-tooling-preview = { group = "androidx.compose.ui", name = "ui-tooling-preview" }
androidx-compose-ui-tooling = { group = "androidx.compose.ui", name = "ui-tooling" }
androidx-compose-material3 = { group = "androidx.compose.material3", name = "material3" }
androidx-documentfile = { group = "androidx.documentfile", name = "documentfile", version = "1.0.1" }

kotlinx-coroutines-android = { group = "org.jetbrains.kotlinx", name = "kotlinx-coroutines-android", version.ref = "coroutines" }
kotlinx-coroutines-test = { group = "org.jetbrains.kotlinx", name = "kotlinx-coroutines-test", version.ref = "coroutines" }

ktor-server-core = { group = "io.ktor", name = "ktor-server-core", version.ref = "ktor" }
ktor-server-cio = { group = "io.ktor", name = "ktor-server-cio", version.ref = "ktor" }
ktor-server-test-host = { group = "io.ktor", name = "ktor-server-test-host", version.ref = "ktor" }
ktor-client-core = { group = "io.ktor", name = "ktor-client-core", version.ref = "ktor" }
ktor-client-cio = { group = "io.ktor", name = "ktor-client-cio", version.ref = "ktor" }
ktor-client-mock = { group = "io.ktor", name = "ktor-client-mock", version.ref = "ktor" }

commons-compress = { group = "org.apache.commons", name = "commons-compress", version.ref = "commonsCompress" }

junit = { group = "junit", name = "junit", version.ref = "junit" }

[plugins]
android-application = { id = "com.android.application", version.ref = "agp" }
kotlin-android = { id = "org.jetbrains.kotlin.android", version.ref = "kotlin" }
kotlin-compose = { id = "org.jetbrains.kotlin.plugin.compose", version.ref = "kotlin" }
```

- [ ] **Step 1.4: Create `android-app/settings.gradle.kts`**

```kotlin
pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}
dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        google()
        mavenCentral()
    }
}

rootProject.name = "ftr"
include(":app")
```

- [ ] **Step 1.5: Create `android-app/build.gradle.kts` (top-level)**

```kotlin
plugins {
    alias(libs.plugins.android.application) apply false
    alias(libs.plugins.kotlin.android) apply false
    alias(libs.plugins.kotlin.compose) apply false
}
```

- [ ] **Step 1.6: Create `android-app/gradle/wrapper/gradle-wrapper.properties`**

```properties
distributionBase=GRADLE_USER_HOME
distributionPath=wrapper/dists
distributionUrl=https\://services.gradle.org/distributions/gradle-8.9-bin.zip
networkTimeout=10000
validateDistributionUrl=true
zipStoreBase=GRADLE_USER_HOME
zipStorePath=wrapper/dists
```

- [ ] **Step 1.7: Generate the Gradle wrapper scripts**

Run:
```
cd android-app && gradle wrapper --gradle-version 8.9
```
Expected: `gradlew`, `gradlew.bat`, and `gradle/wrapper/gradle-wrapper.jar` created. If the system `gradle` isn't available, install it (`sdk install gradle 8.9` or download binary) — only needed once to bootstrap the wrapper.

Then make `gradlew` executable:
```
chmod +x android-app/gradlew
```

- [ ] **Step 1.8: Create `android-app/app/build.gradle.kts`**

```kotlin
plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.android)
    alias(libs.plugins.kotlin.compose)
}

android {
    namespace = "com.filetransfer.ftr"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.filetransfer.ftr"
        minSdk = 26
        targetSdk = 34
        versionCode = 1
        versionName = "0.1.0"

        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro"
            )
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions { jvmTarget = "17" }

    buildFeatures { compose = true }

    packaging {
        resources {
            excludes += "/META-INF/{AL2.0,LGPL2.1,INDEX.LIST,DEPENDENCIES}"
        }
    }

    testOptions {
        unitTests {
            isIncludeAndroidResources = true
            isReturnDefaultValues = true
        }
    }
}

dependencies {
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.lifecycle.runtime.ktx)
    implementation(libs.androidx.lifecycle.viewmodel.compose)
    implementation(libs.androidx.activity.compose)
    implementation(platform(libs.androidx.compose.bom))
    implementation(libs.androidx.compose.ui)
    implementation(libs.androidx.compose.ui.tooling.preview)
    implementation(libs.androidx.compose.material3)
    implementation(libs.androidx.documentfile)

    implementation(libs.kotlinx.coroutines.android)

    implementation(libs.ktor.server.core)
    implementation(libs.ktor.server.cio)
    implementation(libs.ktor.client.core)
    implementation(libs.ktor.client.cio)

    implementation(libs.commons.compress)

    debugImplementation(libs.androidx.compose.ui.tooling)

    testImplementation(libs.junit)
    testImplementation(libs.kotlinx.coroutines.test)
    testImplementation(libs.ktor.server.test.host)
    testImplementation(libs.ktor.client.mock)
}
```

- [ ] **Step 1.9: Create `android-app/app/proguard-rules.pro`**

```
# Keep Ktor reflection targets
-keep class io.ktor.** { *; }
-dontwarn io.ktor.**
# Keep Commons Compress
-keep class org.apache.commons.compress.** { *; }
```

- [ ] **Step 1.10: Create `android-app/app/src/main/AndroidManifest.xml`**

```xml
<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android">

    <uses-permission android:name="android.permission.INTERNET" />
    <uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />
    <uses-permission android:name="android.permission.CHANGE_WIFI_MULTICAST_STATE" />

    <application
        android:label="@string/app_name"
        android:theme="@style/Theme.FileTransfer"
        android:allowBackup="true"
        android:supportsRtl="true">

        <activity
            android:name=".MainActivity"
            android:exported="true">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>

    </application>

</manifest>
```

- [ ] **Step 1.11: Create `android-app/app/src/main/res/values/strings.xml`**

```xml
<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="app_name">FileTransfer</string>
</resources>
```

- [ ] **Step 1.12: Create `android-app/app/src/main/res/values/themes.xml`**

```xml
<?xml version="1.0" encoding="utf-8"?>
<resources>
    <style name="Theme.FileTransfer" parent="android:Theme.Material.Light.NoActionBar" />
</resources>
```

- [ ] **Step 1.13: Create `android-app/app/src/main/java/com/filetransfer/ftr/MainActivity.kt` (minimal stub, finalized in Task 9)**

```kotlin
package com.filetransfer.ftr

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            MaterialTheme {
                Surface(modifier = Modifier.fillMaxSize()) {
                    Box(contentAlignment = Alignment.Center, modifier = Modifier.fillMaxSize()) {
                        Text("FileTransfer")
                    }
                }
            }
        }
    }
}
```

- [ ] **Step 1.14: Build the debug APK to prove the scaffold works**

Run:
```
cd android-app && ./gradlew assembleDebug
```
Expected: `BUILD SUCCESSFUL`. APK at `app/build/outputs/apk/debug/app-debug.apk`. If the first run fails with "SDK location not found", create `android-app/local.properties` with `sdk.dir=/path/to/Android/Sdk`.

- [ ] **Step 1.15: Commit**

```
git add android-app/.gitignore \
        android-app/settings.gradle.kts \
        android-app/build.gradle.kts \
        android-app/gradle.properties \
        android-app/gradle/libs.versions.toml \
        android-app/gradle/wrapper/gradle-wrapper.properties \
        android-app/gradle/wrapper/gradle-wrapper.jar \
        android-app/gradlew \
        android-app/gradlew.bat \
        android-app/app/build.gradle.kts \
        android-app/app/proguard-rules.pro \
        android-app/app/src/main/AndroidManifest.xml \
        android-app/app/src/main/res \
        android-app/app/src/main/java
git commit -m "feat(android): scaffold Gradle project with empty Compose activity"
```

---

## Task 2: `Peer` model and `PassKey` utility

**Goal:** Data classes and small helpers that later tasks depend on. No networking logic.

**Files:**
- Create: `android-app/app/src/main/java/com/filetransfer/ftr/model/Peer.kt`
- Create: `android-app/app/src/main/java/com/filetransfer/ftr/util/PassKey.kt`

- [ ] **Step 2.1: Create `Peer.kt`**

```kotlin
package com.filetransfer.ftr.model

/** A discovered ftr peer on the LAN. Mirrors ios-app Peer.swift. */
data class Peer(
    val id: String,       // unique key for list diffing: "<name>.<type>.<domain>"
    val name: String,
    val host: String,
    val port: Int,
    val dropDir: String   // from the mDNS TXT record
)
```

- [ ] **Step 2.2: Create `PassKey.kt`**

```kotlin
package com.filetransfer.ftr.util

import java.security.SecureRandom

object PassKey {
    private const val ALPHABET =
        "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    private val random = SecureRandom()

    /** Generates a random N-character passkey. Default length matches main.go's randomPassKey(6). */
    fun random(length: Int = 6): String {
        val chars = CharArray(length)
        for (i in 0 until length) {
            chars[i] = ALPHABET[random.nextInt(ALPHABET.length)]
        }
        return String(chars)
    }
}
```

- [ ] **Step 2.3: Build to confirm nothing breaks**

Run: `cd android-app && ./gradlew assembleDebug`
Expected: `BUILD SUCCESSFUL`.

- [ ] **Step 2.4: Commit**

```
git add android-app/app/src/main/java/com/filetransfer/ftr/model/Peer.kt \
        android-app/app/src/main/java/com/filetransfer/ftr/util/PassKey.kt
git commit -m "feat(android): add Peer model and PassKey util"
```

---

## Task 3: `TarGz` — tar.gz compress/decompress with TDD

**Goal:** Pure JVM wrapper around Commons Compress matching the on-the-wire format that Go's `archive/tar` + `compress/gzip` produces. Zip-slip safe. Directory permission fix-up. Tested on JVM only (no Android deps).

**Files:**
- Create: `android-app/app/src/test/java/com/filetransfer/ftr/archive/TarGzTest.kt`
- Create: `android-app/app/src/main/java/com/filetransfer/ftr/archive/TarGz.kt`

- [ ] **Step 3.1: Write the round-trip + zip-slip failing tests**

Create `android-app/app/src/test/java/com/filetransfer/ftr/archive/TarGzTest.kt`:

```kotlin
package com.filetransfer.ftr.archive

import org.apache.commons.compress.archivers.tar.TarArchiveEntry
import org.apache.commons.compress.archivers.tar.TarArchiveOutputStream
import org.apache.commons.compress.compressors.gzip.GzipCompressorOutputStream
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

class TarGzTest {
    @get:Rule val tmp = TemporaryFolder()

    @Test fun roundTrip_filesAndDirectories() {
        val src = tmp.newFolder("src")
        File(src, "a.txt").writeText("hello")
        val sub = File(src, "sub").apply { mkdirs() }
        File(sub, "b.bin").writeBytes(byteArrayOf(0, 1, 2, 3))

        val tarball = tmp.newFile("src.tar.gz").also { it.delete() }
        TarGz.compressDirectory(src, tarball)

        val out = tmp.newFolder("out")
        TarGz.decompress(tarball, out)

        assertEquals("hello", File(out, "a.txt").readText())
        assertArrayEquals(byteArrayOf(0, 1, 2, 3), File(out, "sub/b.bin").readBytes())
        assertTrue(File(out, "sub").isDirectory)
    }

    @Test fun zipSlip_isRejected() {
        // Hand-craft a tarball with a ../evil.txt entry
        val tarball = tmp.newFile("evil.tar.gz").also { it.delete() }
        tarball.outputStream().use { fos ->
            GzipCompressorOutputStream(fos).use { gzip ->
                TarArchiveOutputStream(gzip).use { tar ->
                    tar.setLongFileMode(TarArchiveOutputStream.LONGFILE_POSIX)
                    val entry = TarArchiveEntry("../evil.txt")
                    val payload = "pwn".toByteArray()
                    entry.size = payload.size.toLong()
                    tar.putArchiveEntry(entry)
                    tar.write(payload)
                    tar.closeArchiveEntry()
                }
            }
        }
        val out = tmp.newFolder("out")
        var threw = false
        try {
            TarGz.decompress(tarball, out)
        } catch (e: TarGz.ZipSlipException) {
            threw = true
        }
        assertTrue("expected ZipSlipException", threw)
        assertFalse(File(out.parentFile, "evil.txt").exists())
    }

    @Test fun topLevelDirectory_isNotIncluded() {
        // Go's zipTar skips the top-level directory entry; we do too.
        val src = tmp.newFolder("top")
        File(src, "only.txt").writeText("hi")
        val tarball = tmp.newFile("top.tar.gz").also { it.delete() }
        TarGz.compressDirectory(src, tarball)

        val names = mutableListOf<String>()
        org.apache.commons.compress.compressors.gzip.GzipCompressorInputStream(
            tarball.inputStream()
        ).use { gzi ->
            org.apache.commons.compress.archivers.tar.TarArchiveInputStream(gzi).use { tar ->
                var e = tar.nextTarEntry
                while (e != null) {
                    names += e.name
                    e = tar.nextTarEntry
                }
            }
        }
        // Should NOT contain "top/" or "./"
        assertFalse("top-level dir must not be an entry", names.any { it == "top/" || it == "./" })
        assertTrue(names.contains("only.txt"))
    }
}
```

- [ ] **Step 3.2: Run tests to confirm they fail (TarGz doesn't exist yet)**

Run: `cd android-app && ./gradlew :app:testDebugUnitTest --tests "com.filetransfer.ftr.archive.TarGzTest"`
Expected: compilation failure — `Unresolved reference: TarGz`. Good; that's the red state.

- [ ] **Step 3.3: Create `TarGz.kt` with the minimal implementation to satisfy the tests**

```kotlin
package com.filetransfer.ftr.archive

import org.apache.commons.compress.archivers.tar.TarArchiveEntry
import org.apache.commons.compress.archivers.tar.TarArchiveInputStream
import org.apache.commons.compress.archivers.tar.TarArchiveOutputStream
import org.apache.commons.compress.compressors.gzip.GzipCompressorInputStream
import org.apache.commons.compress.compressors.gzip.GzipCompressorOutputStream
import java.io.File
import java.io.IOException

/**
 * Compresses and decompresses tar.gz archives in the exact format produced by
 * the Go CLI (`main.go` zipTar/unzipUntar) so both sides can interoperate.
 *
 * Rules enforced here:
 *  - Top-level directory is NOT emitted as an entry; paths are relative to src.
 *  - Symlinks are skipped entirely (not followed, not archived).
 *  - Directory permission fix-up: if any read bit is set, the matching execute
 *    bit is set too, so the receiver can traverse the dir after extraction.
 *  - Extraction refuses any entry whose resolved path escapes the destination
 *    directory (zip-slip guard).
 */
object TarGz {

    class ZipSlipException(name: String) :
        IOException("Entry '$name' would escape destination directory")

    /**
     * Walks `src` recursively and writes its contents to `tarball` as a
     * POSIX ustar archive wrapped in gzip. `src` itself is not included as
     * an entry (only its contents, with relative paths).
     */
    fun compressDirectory(src: File, tarball: File) {
        require(src.isDirectory) { "source is not a directory: $src" }
        tarball.outputStream().buffered().use { fos ->
            GzipCompressorOutputStream(fos).use { gzip ->
                TarArchiveOutputStream(gzip).use { tar ->
                    tar.setLongFileMode(TarArchiveOutputStream.LONGFILE_POSIX)
                    tar.setBigNumberMode(TarArchiveOutputStream.BIGNUMBER_POSIX)

                    src.walkTopDown().forEach { file ->
                        if (file == src) return@forEach // skip top-level dir entry
                        if (isSymlink(file)) return@forEach

                        val rel = src.toPath().relativize(file.toPath()).toString()
                            .replace(File.separatorChar, '/')

                        val name = if (file.isDirectory) "$rel/" else rel
                        val entry = TarArchiveEntry(file, name)

                        // Directory permission fix-up (mirrors main.go:236-244)
                        if (file.isDirectory) {
                            entry.mode = fixupDirMode(entry.mode)
                        }

                        tar.putArchiveEntry(entry)
                        if (file.isFile) {
                            file.inputStream().use { it.copyTo(tar) }
                        }
                        tar.closeArchiveEntry()
                    }
                }
            }
        }
    }

    /**
     * Decompresses `tarball` into `destDir`, which must already exist. Refuses
     * entries whose resolved paths escape `destDir` (zip-slip protection).
     */
    fun decompress(tarball: File, destDir: File) {
        require(destDir.isDirectory) { "destination is not a directory: $destDir" }
        val destCanonical = destDir.canonicalPath + File.separator

        tarball.inputStream().buffered().use { fis ->
            GzipCompressorInputStream(fis).use { gzi ->
                TarArchiveInputStream(gzi).use { tar ->
                    var entry = tar.nextTarEntry
                    while (entry != null) {
                        val outFile = File(destDir, entry.name)
                        val outCanonical = outFile.canonicalPath
                        if (!outCanonical.startsWith(destCanonical) &&
                            outCanonical != destDir.canonicalPath
                        ) {
                            throw ZipSlipException(entry.name)
                        }

                        if (entry.isDirectory) {
                            if (!outFile.exists() && !outFile.mkdirs()) {
                                throw IOException("failed to mkdir $outFile")
                            }
                        } else {
                            outFile.parentFile?.let { p ->
                                if (!p.exists() && !p.mkdirs()) {
                                    throw IOException("failed to mkdir $p")
                                }
                            }
                            outFile.outputStream().use { os -> tar.copyTo(os) }
                        }
                        entry = tar.nextTarEntry
                    }
                }
            }
        }
    }

    /** Matches main.go:236-244: ensure any readable bit has a matching execute bit. */
    private fun fixupDirMode(mode: Int): Int {
        var m = mode
        if ((m and 0b100_000_000) != 0) m = m or 0b001_000_000 // owner r -> owner x
        if ((m and 0b000_100_000) != 0) m = m or 0b000_001_000 // group r -> group x
        if ((m and 0b000_000_100) != 0) m = m or 0b000_000_001 // other r -> other x
        return m
    }

    private fun isSymlink(f: File): Boolean =
        try { java.nio.file.Files.isSymbolicLink(f.toPath()) } catch (_: Exception) { false }
}
```

- [ ] **Step 3.4: Run tests to verify they pass**

Run: `cd android-app && ./gradlew :app:testDebugUnitTest --tests "com.filetransfer.ftr.archive.TarGzTest"`
Expected: `BUILD SUCCESSFUL`, all three tests pass. If `zipSlip_isRejected` fails, check the canonical-path prefix comparison — on some JDKs you need the explicit separator check.

- [ ] **Step 3.5: Commit**

```
git add android-app/app/src/main/java/com/filetransfer/ftr/archive/TarGz.kt \
        android-app/app/src/test/java/com/filetransfer/ftr/archive/TarGzTest.kt
git commit -m "feat(android): add TarGz with round-trip, zip-slip, top-level dir tests"
```

---

## Task 4: `NsdAdvertiser` + TXT-record encoding test

**Goal:** Register an `_ftr._tcp.local.` service via `NsdManager` with a TXT record whose single attribute key is the drop-dir path and whose value is empty. Verify the encoding against a Go-produced fixture.

**Files:**
- Create: `android-app/app/src/test/resources/fixtures/go-zeroconf-txt.bin`
- Create: `android-app/app/src/test/java/com/filetransfer/ftr/net/NsdAdvertiserTxtRecordTest.kt`
- Create: `android-app/app/src/main/java/com/filetransfer/ftr/net/NsdAdvertiser.kt`

- [ ] **Step 4.1: Generate the Go TXT fixture**

The fixture is the raw TXT RDATA bytes that `grandcat/zeroconf` emits for a service whose text record is `[]string{"/storage/emulated/0/Download"}`. DNS TXT record format: each string is a length-prefixed byte sequence. So the expected bytes are:
```
[0x1d] /storage/emulated/0/Download
```
(0x1d = 29 decimal = length of the path).

Write that exact byte sequence to `android-app/app/src/test/resources/fixtures/go-zeroconf-txt.bin`. From the repo root:

```
mkdir -p android-app/app/src/test/resources/fixtures
printf '\x1d/storage/emulated/0/Download' > android-app/app/src/test/resources/fixtures/go-zeroconf-txt.bin
```

Verify size:
```
wc -c android-app/app/src/test/resources/fixtures/go-zeroconf-txt.bin
```
Expected: `30` (1 length byte + 29 ASCII bytes).

- [ ] **Step 4.2: Write the failing TXT-record test**

Create `android-app/app/src/test/java/com/filetransfer/ftr/net/NsdAdvertiserTxtRecordTest.kt`:

```kotlin
package com.filetransfer.ftr.net

import org.junit.Assert.assertArrayEquals
import org.junit.Test

class NsdAdvertiserTxtRecordTest {

    @Test fun encodesTxtLikeGoZeroconf() {
        val dropDir = "/storage/emulated/0/Download"
        val encoded = NsdAdvertiser.encodeTxtRdata(dropDir)

        // Load the Go fixture from the classpath
        val expected = javaClass.classLoader!!
            .getResourceAsStream("fixtures/go-zeroconf-txt.bin")!!
            .use { it.readBytes() }

        assertArrayEquals(expected, encoded)
    }
}
```

- [ ] **Step 4.3: Run the test to confirm it fails**

Run: `cd android-app && ./gradlew :app:testDebugUnitTest --tests "com.filetransfer.ftr.net.NsdAdvertiserTxtRecordTest"`
Expected: compilation failure, `Unresolved reference: NsdAdvertiser`.

- [ ] **Step 4.4: Implement `NsdAdvertiser.kt`**

Create `android-app/app/src/main/java/com/filetransfer/ftr/net/NsdAdvertiser.kt`:

```kotlin
package com.filetransfer.ftr.net

import android.content.Context
import android.net.nsd.NsdManager
import android.net.nsd.NsdServiceInfo
import android.util.Log
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.runBlocking

/**
 * Registers an `_ftr._tcp` service via NsdManager so other ftr peers can find
 * this device. The TXT record carries a single attribute whose key is the
 * drop-dir path and whose value is empty — matching Go's zeroconf.Register(...)
 * behavior in main.go:137 and iOS NWTXTRecord([dropDir: ""]) in FileReceiver.swift:29.
 */
class NsdAdvertiser(private val context: Context) {

    companion object {
        private const val TAG = "NsdAdvertiser"
        const val SERVICE_TYPE = "_ftr._tcp."

        /**
         * Encodes a single drop-dir path into a DNS TXT RDATA byte sequence
         * exactly as Go's grandcat/zeroconf does: one length-prefixed string,
         * where the string is the path itself (no key=value format).
         *
         * Exposed for unit testing against a Go-produced fixture.
         */
        fun encodeTxtRdata(dropDir: String): ByteArray {
            val bytes = dropDir.toByteArray(Charsets.UTF_8)
            require(bytes.size <= 255) { "drop dir too long for a single TXT string" }
            val out = ByteArray(bytes.size + 1)
            out[0] = bytes.size.toByte()
            System.arraycopy(bytes, 0, out, 1, bytes.size)
            return out
        }
    }

    private val nsd by lazy { context.getSystemService(Context.NSD_SERVICE) as NsdManager }
    private var registrationListener: NsdManager.RegistrationListener? = null

    suspend fun register(name: String, port: Int, dropDir: String) {
        val info = NsdServiceInfo().apply {
            serviceName = name
            serviceType = SERVICE_TYPE
            this.port = port
            // setAttribute(key, null) serializes as a key-only string — matches
            // the Go fixture verified by NsdAdvertiserTxtRecordTest.
            setAttribute(dropDir, null)
        }

        val ready = Channel<Result<Unit>>(capacity = 1)
        val listener = object : NsdManager.RegistrationListener {
            override fun onRegistrationFailed(si: NsdServiceInfo, errorCode: Int) {
                Log.w(TAG, "registration failed: $errorCode")
                ready.trySend(Result.failure(RuntimeException("NSD registration failed: $errorCode")))
            }
            override fun onUnregistrationFailed(si: NsdServiceInfo, errorCode: Int) {
                Log.w(TAG, "unregistration failed: $errorCode")
            }
            override fun onServiceRegistered(si: NsdServiceInfo) {
                Log.i(TAG, "registered: ${si.serviceName}")
                ready.trySend(Result.success(Unit))
            }
            override fun onServiceUnregistered(si: NsdServiceInfo) {
                Log.i(TAG, "unregistered: ${si.serviceName}")
            }
        }
        registrationListener = listener
        nsd.registerService(info, NsdManager.PROTOCOL_DNS_SD, listener)
        ready.receive().getOrThrow()
    }

    fun stop() {
        registrationListener?.let { l ->
            runCatching { nsd.unregisterService(l) }
            registrationListener = null
        }
    }
}
```

- [ ] **Step 4.5: Run the test to verify it passes**

Run: `cd android-app && ./gradlew :app:testDebugUnitTest --tests "com.filetransfer.ftr.net.NsdAdvertiserTxtRecordTest"`
Expected: PASS.

If the bytes don't match, print both arrays as hex in the failure message and check:
- Length prefix byte is unsigned (use `.toByte()` on an `Int`, not a `Short`).
- No trailing null byte (the fixture has exactly 30 bytes, no more).

- [ ] **Step 4.6: Run the full unit test suite as a smoke check**

Run: `cd android-app && ./gradlew :app:testDebugUnitTest`
Expected: `BUILD SUCCESSFUL`, all tests pass.

- [ ] **Step 4.7: Commit**

```
git add android-app/app/src/main/java/com/filetransfer/ftr/net/NsdAdvertiser.kt \
        android-app/app/src/test/java/com/filetransfer/ftr/net/NsdAdvertiserTxtRecordTest.kt \
        android-app/app/src/test/resources/fixtures/go-zeroconf-txt.bin
git commit -m "feat(android): add NsdAdvertiser with Go-compatible TXT encoding"
```

---

## Task 5: `NsdBrowser`

**Goal:** Discover `_ftr._tcp.local.` peers via `NsdManager`, resolve each to get host/port/TXT, expose them as a `StateFlow<List<Peer>>`. No unit test — this is all Android framework calls; integration-verified via manual testing in Task 13.

**Files:**
- Create: `android-app/app/src/main/java/com/filetransfer/ftr/net/NsdBrowser.kt`

- [ ] **Step 5.1: Create `NsdBrowser.kt`**

```kotlin
package com.filetransfer.ftr.net

import android.content.Context
import android.net.nsd.NsdManager
import android.net.nsd.NsdServiceInfo
import android.net.wifi.WifiManager
import android.util.Log
import com.filetransfer.ftr.model.Peer
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

/**
 * Browses for `_ftr._tcp` services on the LAN and maintains a reactive list
 * of discovered peers. Holds a WifiManager.MulticastLock while active so
 * multicast DNS packets reach the app.
 */
class NsdBrowser(private val context: Context) {

    companion object { private const val TAG = "NsdBrowser" }

    private val nsd by lazy { context.getSystemService(Context.NSD_SERVICE) as NsdManager }
    private val wifi by lazy { context.applicationContext.getSystemService(Context.WIFI_SERVICE) as WifiManager }

    private val _peers = MutableStateFlow<List<Peer>>(emptyList())
    val peers: StateFlow<List<Peer>> = _peers.asStateFlow()

    private val _isSearching = MutableStateFlow(false)
    val isSearching: StateFlow<Boolean> = _isSearching.asStateFlow()

    private var multicastLock: WifiManager.MulticastLock? = null
    private var discoveryListener: NsdManager.DiscoveryListener? = null

    fun start() {
        if (discoveryListener != null) return

        multicastLock = wifi.createMulticastLock("ftr-nsd").apply {
            setReferenceCounted(false)
            acquire()
        }

        val listener = object : NsdManager.DiscoveryListener {
            override fun onStartDiscoveryFailed(serviceType: String, errorCode: Int) {
                Log.w(TAG, "onStartDiscoveryFailed: $errorCode")
                releaseLock()
                _isSearching.value = false
            }
            override fun onStopDiscoveryFailed(serviceType: String, errorCode: Int) {
                Log.w(TAG, "onStopDiscoveryFailed: $errorCode")
            }
            override fun onDiscoveryStarted(serviceType: String) {
                _isSearching.value = true
            }
            override fun onDiscoveryStopped(serviceType: String) {
                _isSearching.value = false
            }
            override fun onServiceFound(service: NsdServiceInfo) {
                // Must resolve to get host/port/TXT
                nsd.resolveService(service, makeResolveListener())
            }
            override fun onServiceLost(service: NsdServiceInfo) {
                val id = idFor(service)
                _peers.value = _peers.value.filterNot { it.id == id }
            }
        }
        discoveryListener = listener
        nsd.discoverServices(NsdAdvertiser.SERVICE_TYPE, NsdManager.PROTOCOL_DNS_SD, listener)
    }

    fun stop() {
        discoveryListener?.let { l ->
            runCatching { nsd.stopServiceDiscovery(l) }
            discoveryListener = null
        }
        _peers.value = emptyList()
        _isSearching.value = false
        releaseLock()
    }

    private fun releaseLock() {
        multicastLock?.let { if (it.isHeld) runCatching { it.release() } }
        multicastLock = null
    }

    private fun idFor(si: NsdServiceInfo): String =
        "${si.serviceName}.${si.serviceType}"

    private fun makeResolveListener() = object : NsdManager.ResolveListener {
        override fun onResolveFailed(service: NsdServiceInfo, errorCode: Int) {
            Log.w(TAG, "resolve failed for ${service.serviceName}: $errorCode")
        }
        override fun onServiceResolved(service: NsdServiceInfo) {
            val host = service.host?.hostAddress ?: return
            val dropDir = service.attributes.keys.firstOrNull() ?: ""
            val peer = Peer(
                id = idFor(service),
                name = service.serviceName,
                host = host,
                port = service.port,
                dropDir = dropDir
            )
            val current = _peers.value
            if (current.none { it.id == peer.id }) {
                _peers.value = current + peer
            }
        }
    }
}
```

- [ ] **Step 5.2: Build to confirm compilation**

Run: `cd android-app && ./gradlew assembleDebug`
Expected: `BUILD SUCCESSFUL`.

- [ ] **Step 5.3: Commit**

```
git add android-app/app/src/main/java/com/filetransfer/ftr/net/NsdBrowser.kt
git commit -m "feat(android): add NsdBrowser with multicast lock and peer StateFlow"
```

---

## Task 6: `FileReceiver` — embedded Ktor server + `/upload` handler

**Goal:** Start/stop a Ktor CIO server on `0.0.0.0:<port>`, handle `POST /upload` with passkey auth, multipart file parsing, 409-on-conflict, and tar extraction when `X-Ftr-File-Type: dir`. Tested with `testApplication { ... }` from Ktor's test host.

**Files:**
- Create: `android-app/app/src/main/java/com/filetransfer/ftr/net/FileReceiver.kt`
- Create: `android-app/app/src/test/java/com/filetransfer/ftr/net/FileReceiverTest.kt`

- [ ] **Step 6.1: Write failing tests for the route logic**

Create `android-app/app/src/test/java/com/filetransfer/ftr/net/FileReceiverTest.kt`:

```kotlin
package com.filetransfer.ftr.net

import io.ktor.client.request.forms.MultiPartFormDataContent
import io.ktor.client.request.forms.formData
import io.ktor.client.request.header
import io.ktor.client.request.post
import io.ktor.client.request.setBody
import io.ktor.http.Headers
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpStatusCode
import io.ktor.server.testing.testApplication
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

class FileReceiverTest {
    @get:Rule val tmp = TemporaryFolder()

    private fun dropDir(): File = tmp.newFolder("drop")

    @Test fun rejectsWrongPasskey() = testApplication {
        val drop = dropDir()
        application { FileReceiver.installRoutes(this, drop, passKey = "secret") }
        val resp = client.post("/upload") {
            header("X-Ftr-Passkey", "wrong")
            header("X-Ftr-File-Type", "file")
            setBody(MultiPartFormDataContent(formData {
                append("file", "hi".toByteArray(), Headers.build {
                    append(HttpHeaders.ContentDisposition, "filename=\"a.txt\"")
                })
            }))
        }
        assertEquals(HttpStatusCode.Unauthorized, resp.status)
        assertTrue(drop.listFiles().isNullOrEmpty())
    }

    @Test fun writesSingleFile() = testApplication {
        val drop = dropDir()
        application { FileReceiver.installRoutes(this, drop, passKey = "k") }
        val resp = client.post("/upload") {
            header("X-Ftr-Passkey", "k")
            header("X-Ftr-File-Type", "file")
            setBody(MultiPartFormDataContent(formData {
                append("file", "hello".toByteArray(), Headers.build {
                    append(HttpHeaders.ContentDisposition, "filename=\"greeting.txt\"")
                })
            }))
        }
        assertEquals(HttpStatusCode.OK, resp.status)
        assertEquals("hello", File(drop, "greeting.txt").readText())
    }

    @Test fun refusesOverwrite() = testApplication {
        val drop = dropDir()
        File(drop, "a.txt").writeText("old")
        application { FileReceiver.installRoutes(this, drop, passKey = "k") }
        val resp = client.post("/upload") {
            header("X-Ftr-Passkey", "k")
            header("X-Ftr-File-Type", "file")
            setBody(MultiPartFormDataContent(formData {
                append("file", "new".toByteArray(), Headers.build {
                    append(HttpHeaders.ContentDisposition, "filename=\"a.txt\"")
                })
            }))
        }
        assertEquals(HttpStatusCode.Conflict, resp.status)
        assertEquals("old", File(drop, "a.txt").readText())
    }

    @Test fun extractsDirectoryTarball() = testApplication {
        val drop = dropDir()
        // Build a tar.gz containing "pkg/x.txt"
        val src = tmp.newFolder("src")
        File(src, "x.txt").writeText("xdata")
        val tarball = tmp.newFile("pkg.tar.gz").also { it.delete() }
        com.filetransfer.ftr.archive.TarGz.compressDirectory(src, tarball)

        application { FileReceiver.installRoutes(this, drop, passKey = "k") }
        val resp = client.post("/upload") {
            header("X-Ftr-Passkey", "k")
            header("X-Ftr-File-Type", "dir")
            setBody(MultiPartFormDataContent(formData {
                append("file", tarball.readBytes(), Headers.build {
                    append(HttpHeaders.ContentDisposition, "filename=\"pkg.tar.gz\"")
                })
            }))
        }
        assertEquals(HttpStatusCode.OK, resp.status)
        assertEquals("xdata", File(drop, "pkg/x.txt").readText())
        // The tarball itself must be gone after extraction
        assertTrue(!File(drop, "pkg.tar.gz").exists())
    }
}
```

- [ ] **Step 6.2: Run tests to confirm they fail**

Run: `cd android-app && ./gradlew :app:testDebugUnitTest --tests "com.filetransfer.ftr.net.FileReceiverTest"`
Expected: compilation failure, `Unresolved reference: FileReceiver`.

- [ ] **Step 6.3: Create `FileReceiver.kt`**

```kotlin
package com.filetransfer.ftr.net

import android.content.Context
import android.util.Log
import com.filetransfer.ftr.archive.TarGz
import io.ktor.http.HttpStatusCode
import io.ktor.http.content.PartData
import io.ktor.http.content.forEachPart
import io.ktor.http.content.streamProvider
import io.ktor.server.application.Application
import io.ktor.server.application.call
import io.ktor.server.application.install
import io.ktor.server.cio.CIO
import io.ktor.server.engine.embeddedServer
import io.ktor.server.engine.ApplicationEngine
import io.ktor.server.request.header
import io.ktor.server.request.receiveMultipart
import io.ktor.server.response.respond
import io.ktor.server.routing.Routing
import io.ktor.server.routing.post
import io.ktor.server.routing.routing
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import java.io.File

/**
 * Hosts a tiny HTTP server exposing POST /upload, matching the Go CLI's
 * receiver protocol in main.go. Lifecycle is activity-scoped: start() when
 * the user taps "Join Network", stop() when they leave.
 */
class FileReceiver(private val context: Context) {

    companion object {
        private const val TAG = "FileReceiver"
        private const val PASSKEY_HEADER = "X-Ftr-Passkey"
        private const val FILETYPE_HEADER = "X-Ftr-File-Type"

        /** Installed into an Application — extracted so tests can reuse it. */
        fun installRoutes(app: Application, dropDir: File, passKey: String) {
            app.routing {
                post("/upload") {
                    val callPassKey = call.request.header(PASSKEY_HEADER) ?: ""
                    if (callPassKey != passKey) {
                        call.respond(HttpStatusCode.Unauthorized, "Unauthorized")
                        return@post
                    }
                    val fileType = call.request.header(FILETYPE_HEADER) ?: "file"
                    val multipart = call.receiveMultipart()
                    var handled = false

                    multipart.forEachPart { part ->
                        if (!handled && part is PartData.FileItem && part.name == "file") {
                            val rawName = part.originalFileName ?: ""
                            val name = sanitize(rawName)
                            if (name == null) {
                                call.respond(HttpStatusCode.BadRequest, "Invalid file name")
                                part.dispose(); return@forEachPart
                            }
                            val dst = File(dropDir, name)
                            if (dst.exists()) {
                                call.respond(HttpStatusCode.Conflict, "File already exists")
                                part.dispose(); return@forEachPart
                            }
                            try {
                                dst.outputStream().use { out ->
                                    part.streamProvider().use { it.copyTo(out) }
                                }
                                if (fileType == "dir") {
                                    try {
                                        TarGz.decompress(dst, dropDir)
                                        dst.delete()
                                    } catch (e: Exception) {
                                        Log.w(TAG, "extract failed", e)
                                        dst.delete()
                                        call.respond(
                                            HttpStatusCode.InternalServerError,
                                            "Extract failed: ${e.message}"
                                        )
                                        handled = true
                                        part.dispose(); return@forEachPart
                                    }
                                }
                                call.respond(HttpStatusCode.OK, "OK")
                                handled = true
                            } catch (e: Exception) {
                                Log.w(TAG, "save failed", e)
                                runCatching { dst.delete() }
                                call.respond(
                                    HttpStatusCode.InternalServerError,
                                    "Save failed: ${e.message}"
                                )
                                handled = true
                            }
                        }
                        part.dispose()
                    }

                    if (!handled) {
                        call.respond(HttpStatusCode.BadRequest, "No file part")
                    }
                }
            }
        }

        private fun sanitize(name: String): String? {
            val base = File(name).name // strips any directory prefix
            if (base.isEmpty() || base == "." || base == "..") return null
            return base
        }
    }

    private val _isRunning = MutableStateFlow(false)
    val isRunning: StateFlow<Boolean> = _isRunning.asStateFlow()

    private val _receivedFiles = MutableStateFlow<List<String>>(emptyList())
    val receivedFiles: StateFlow<List<String>> = _receivedFiles.asStateFlow()

    private val _lastError = MutableStateFlow<String?>(null)
    val lastError: StateFlow<String?> = _lastError.asStateFlow()

    private var engine: ApplicationEngine? = null

    val dropDir: File
        get() {
            val base = context.getExternalFilesDir(null)
                ?: throw IllegalStateException("external files dir unavailable")
            return File(base, "drop").apply { if (!exists()) mkdirs() }
        }

    /**
     * Starts the Ktor server. Throws on failure (e.g. port already in use),
     * does not swallow the exception, so callers can surface it in the UI.
     */
    fun start(port: Int, passKey: String) {
        if (engine != null) return
        _lastError.value = null
        val drop = dropDir
        val srv = embeddedServer(CIO, host = "0.0.0.0", port = port) {
            installRoutes(this, drop, passKey)
        }
        try {
            srv.start(wait = false)
        } catch (e: Exception) {
            _lastError.value = e.message
            throw e
        }
        engine = srv
        _isRunning.value = true
    }

    fun stop() {
        engine?.stop(gracePeriodMillis = 500, timeoutMillis = 1000)
        engine = null
        _isRunning.value = false
    }
}
```

- [ ] **Step 6.4: Run tests**

Run: `cd android-app && ./gradlew :app:testDebugUnitTest --tests "com.filetransfer.ftr.net.FileReceiverTest"`
Expected: 4 tests pass. If `extractsDirectoryTarball` fails because `pkg.tar.gz` is not deleted, the handler's post-extraction `dst.delete()` ordering is wrong — fix it so the tarball is removed only after a successful `TarGz.decompress`.

- [ ] **Step 6.5: Run the full suite**

Run: `cd android-app && ./gradlew :app:testDebugUnitTest`
Expected: all tests pass.

- [ ] **Step 6.6: Commit**

```
git add android-app/app/src/main/java/com/filetransfer/ftr/net/FileReceiver.kt \
        android-app/app/src/test/java/com/filetransfer/ftr/net/FileReceiverTest.kt
git commit -m "feat(android): add FileReceiver with Ktor /upload handler and tests"
```

---

## Task 7: `FileSender` — single-file upload

**Goal:** Upload a single file from a `content://` URI (SAF) to a peer via Ktor client. Produces the same multipart/form-data body that the Go receiver expects. Unit-tested with Ktor's `MockEngine`.

**Files:**
- Create: `android-app/app/src/main/java/com/filetransfer/ftr/net/FileSender.kt`
- Create: `android-app/app/src/test/java/com/filetransfer/ftr/net/FileSenderHttpTest.kt`

- [ ] **Step 7.1: Write the failing MockEngine test**

Create `android-app/app/src/test/java/com/filetransfer/ftr/net/FileSenderHttpTest.kt`:

```kotlin
package com.filetransfer.ftr.net

import com.filetransfer.ftr.model.Peer
import io.ktor.client.HttpClient
import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
import io.ktor.http.HttpStatusCode
import io.ktor.http.headersOf
import io.ktor.utils.io.ByteReadChannel
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class FileSenderHttpTest {

    @Test fun sendsFile_withExpectedHeadersAndBody() = runTest {
        var capturedUrl: String? = null
        var capturedPasskey: String? = null
        var capturedFileType: String? = null
        var capturedBody: ByteArray? = null

        val mock = MockEngine { request ->
            capturedUrl = request.url.toString()
            capturedPasskey = request.headers["X-Ftr-Passkey"]
            capturedFileType = request.headers["X-Ftr-File-Type"]
            capturedBody = request.body.toByteArray()
            respond(
                content = ByteReadChannel("OK"),
                status = HttpStatusCode.OK,
                headers = headersOf()
            )
        }
        val client = HttpClient(mock)

        val peer = Peer(id = "p", name = "peer", host = "192.168.1.42", port = 8844, dropDir = "")
        val result = FileSender.uploadBytes(
            client = client,
            peer = peer,
            passKey = "hunter2",
            fileName = "notes.txt",
            bytes = "hello world".toByteArray(),
            isDirectory = false
        )

        assertTrue(result.isSuccess)
        assertEquals("http://192.168.1.42:8844/upload", capturedUrl)
        assertEquals("hunter2", capturedPasskey)
        assertEquals("file", capturedFileType)

        val bodyStr = String(capturedBody!!, Charsets.ISO_8859_1)
        assertTrue("body should contain filename", bodyStr.contains("filename=\"notes.txt\""))
        assertTrue("body should contain form field name", bodyStr.contains("name=\"file\""))
        assertTrue("body should contain raw payload", bodyStr.contains("hello world"))
    }

    // Small helper reused inside the test.
    private suspend fun io.ktor.http.content.OutgoingContent.toByteArray(): ByteArray {
        val buffer = java.io.ByteArrayOutputStream()
        when (val content = this) {
            is io.ktor.http.content.OutgoingContent.ByteArrayContent -> buffer.write(content.bytes())
            is io.ktor.http.content.OutgoingContent.WriteChannelContent -> {
                val channel = io.ktor.utils.io.ByteChannel(autoFlush = true)
                content.writeTo(channel)
                channel.close()
                val bytes = ByteArray(channel.availableForRead)
                channel.readAvailable(bytes)
                buffer.write(bytes)
            }
            is io.ktor.http.content.OutgoingContent.ReadChannelContent -> {
                val ch = content.readFrom()
                val sink = java.io.ByteArrayOutputStream()
                val buf = ByteArray(8192)
                while (true) {
                    val n = ch.readAvailable(buf, 0, buf.size)
                    if (n <= 0) break
                    sink.write(buf, 0, n)
                }
                buffer.write(sink.toByteArray())
            }
            else -> { /* no-op */ }
        }
        return buffer.toByteArray()
    }
}
```

- [ ] **Step 7.2: Run to confirm it fails**

Run: `cd android-app && ./gradlew :app:testDebugUnitTest --tests "com.filetransfer.ftr.net.FileSenderHttpTest"`
Expected: compilation failure, `Unresolved reference: FileSender`.

- [ ] **Step 7.3: Implement `FileSender.kt` (single-file path only; directory support added in Task 8)**

```kotlin
package com.filetransfer.ftr.net

import android.content.Context
import android.net.Uri
import android.provider.OpenableColumns
import com.filetransfer.ftr.model.Peer
import io.ktor.client.HttpClient
import io.ktor.client.engine.cio.CIO
import io.ktor.client.request.forms.MultiPartFormDataContent
import io.ktor.client.request.forms.formData
import io.ktor.client.request.header
import io.ktor.client.request.post
import io.ktor.client.request.setBody
import io.ktor.client.statement.HttpResponse
import io.ktor.http.Headers
import io.ktor.http.HttpHeaders
import io.ktor.http.HttpStatusCode
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.File
import java.io.FileOutputStream

/**
 * Sends a file or directory to an ftr peer using the same multipart/form-data
 * layout and headers as the Go CLI (main.go:496) and iOS app
 * (FileSender.swift:60).
 */
class FileSender(private val context: Context) {

    companion object {
        private const val PASSKEY_HEADER = "X-Ftr-Passkey"
        private const val FILETYPE_HEADER = "X-Ftr-File-Type"

        /**
         * Lower-level entry point used by tests. Sends the given byte payload
         * as the multipart "file" part with the given filename.
         */
        suspend fun uploadBytes(
            client: HttpClient,
            peer: Peer,
            passKey: String,
            fileName: String,
            bytes: ByteArray,
            isDirectory: Boolean
        ): Result<HttpStatusCode> = runCatching {
            val url = "http://${peer.host}:${peer.port}/upload"
            val resp: HttpResponse = client.post(url) {
                header(PASSKEY_HEADER, passKey)
                header(FILETYPE_HEADER, if (isDirectory) "dir" else "file")
                setBody(MultiPartFormDataContent(formData {
                    append("file", bytes, Headers.build {
                        append(HttpHeaders.ContentDisposition, "filename=\"$fileName\"")
                    })
                }))
            }
            resp.status
        }
    }

    /**
     * Send a single file identified by a SAF content:// URI. Reads the file
     * into memory (acceptable for v1; streaming can come later if needed).
     */
    suspend fun sendFile(uri: Uri, peer: Peer, passKey: String): Result<HttpStatusCode> =
        withContext(Dispatchers.IO) {
            runCatching {
                val resolver = context.contentResolver
                val fileName = displayName(uri) ?: "file"
                val bytes = resolver.openInputStream(uri)?.use { it.readBytes() }
                    ?: return@runCatching error("Cannot open $uri")
                HttpClient(CIO).use { client ->
                    uploadBytes(client, peer, passKey, fileName, bytes, isDirectory = false)
                        .getOrThrow()
                }
            }
        }

    private fun displayName(uri: Uri): String? {
        val resolver = context.contentResolver
        resolver.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use { c ->
            if (c.moveToFirst()) {
                val idx = c.getColumnIndex(OpenableColumns.DISPLAY_NAME)
                if (idx >= 0) return c.getString(idx)
            }
        }
        return uri.lastPathSegment
    }
}

private fun error(msg: String): Nothing = throw RuntimeException(msg)
```

- [ ] **Step 7.4: Run the test**

Run: `cd android-app && ./gradlew :app:testDebugUnitTest --tests "com.filetransfer.ftr.net.FileSenderHttpTest"`
Expected: PASS.

- [ ] **Step 7.5: Commit**

```
git add android-app/app/src/main/java/com/filetransfer/ftr/net/FileSender.kt \
        android-app/app/src/test/java/com/filetransfer/ftr/net/FileSenderHttpTest.kt
git commit -m "feat(android): add FileSender single-file upload with MockEngine test"
```

---

## Task 8: `FileSender` — directory send via tar.gz

**Goal:** Extend `FileSender` with a `sendDirectory(treeUri, peer, passKey)` function. Walk a SAF tree URI, materialize its contents into a real temp directory under `context.cacheDir`, call `TarGz.compressDirectory`, send as `X-Ftr-File-Type: dir` with the multipart filename `<dirname>.tar.gz`.

**Files:**
- Modify: `android-app/app/src/main/java/com/filetransfer/ftr/net/FileSender.kt`

- [ ] **Step 8.1: Add `sendDirectory` to `FileSender`**

Insert after `sendFile` and before `displayName`:

```kotlin
    /**
     * Send a SAF directory (from ACTION_OPEN_DOCUMENT_TREE) as a .tar.gz
     * tarball. Walks the tree into a staging dir under cacheDir, tars it,
     * uploads it, then cleans up.
     */
    suspend fun sendDirectory(treeUri: Uri, peer: Peer, passKey: String): Result<HttpStatusCode> =
        withContext(Dispatchers.IO) {
            runCatching {
                val tree = androidx.documentfile.provider.DocumentFile.fromTreeUri(context, treeUri)
                    ?: return@runCatching error("Cannot open tree $treeUri")
                val dirName = tree.name ?: "folder"

                val staging = File(context.cacheDir, "ftr-send-${System.currentTimeMillis()}")
                val stagedRoot = File(staging, dirName).apply { mkdirs() }
                try {
                    copyTreeToDisk(tree, stagedRoot)
                    val tarball = File(staging, "$dirName.tar.gz")
                    com.filetransfer.ftr.archive.TarGz.compressDirectory(stagedRoot, tarball)

                    HttpClient(CIO).use { client ->
                        uploadBytes(
                            client = client,
                            peer = peer,
                            passKey = passKey,
                            fileName = "$dirName.tar.gz",
                            bytes = tarball.readBytes(),
                            isDirectory = true
                        ).getOrThrow()
                    }
                } finally {
                    staging.deleteRecursively()
                }
            }
        }

    private fun copyTreeToDisk(
        tree: androidx.documentfile.provider.DocumentFile,
        dest: File
    ) {
        if (!dest.exists()) dest.mkdirs()
        for (child in tree.listFiles()) {
            val childDest = File(dest, child.name ?: continue)
            if (child.isDirectory) {
                copyTreeToDisk(child, childDest)
            } else if (child.isFile) {
                context.contentResolver.openInputStream(child.uri)?.use { input ->
                    FileOutputStream(childDest).use { out -> input.copyTo(out) }
                }
            }
        }
    }
```

Also ensure the existing imports at the top of `FileSender.kt` include `java.io.FileOutputStream` (already added in Task 7.3).

- [ ] **Step 8.2: Build to confirm compilation**

Run: `cd android-app && ./gradlew assembleDebug`
Expected: `BUILD SUCCESSFUL`.

(No unit test — `DocumentFile.fromTreeUri` is an Android-only API that can't run on the JVM. Covered by the manual interop matrix in Task 13.)

- [ ] **Step 8.3: Commit**

```
git add android-app/app/src/main/java/com/filetransfer/ftr/net/FileSender.kt
git commit -m "feat(android): add FileSender directory send via tar.gz"
```

---

## Task 9: `FtrApplication`, theme, `MainScreen` TabRow, wire up `MainActivity`

**Goal:** Application-scoped singletons for `NsdBrowser` / `FileReceiver` / `FileSender`, a Material3 theme, and a `MainScreen` with two tabs ("Send" and "Receive") that host placeholder content until Tasks 10 and 11 fill them in.

**Files:**
- Create: `android-app/app/src/main/java/com/filetransfer/ftr/FtrApplication.kt`
- Create: `android-app/app/src/main/java/com/filetransfer/ftr/ui/theme/Color.kt`
- Create: `android-app/app/src/main/java/com/filetransfer/ftr/ui/theme/Type.kt`
- Create: `android-app/app/src/main/java/com/filetransfer/ftr/ui/theme/Theme.kt`
- Create: `android-app/app/src/main/java/com/filetransfer/ftr/ui/MainScreen.kt`
- Modify: `android-app/app/src/main/java/com/filetransfer/ftr/MainActivity.kt`
- Modify: `android-app/app/src/main/AndroidManifest.xml` (register `FtrApplication`)

- [ ] **Step 9.1: Create `FtrApplication.kt`**

```kotlin
package com.filetransfer.ftr

import android.app.Application
import com.filetransfer.ftr.net.FileReceiver
import com.filetransfer.ftr.net.FileSender
import com.filetransfer.ftr.net.NsdAdvertiser
import com.filetransfer.ftr.net.NsdBrowser

class FtrApplication : Application() {
    val nsdBrowser by lazy { NsdBrowser(this) }
    val nsdAdvertiser by lazy { NsdAdvertiser(this) }
    val fileReceiver by lazy { FileReceiver(this) }
    val fileSender by lazy { FileSender(this) }
}
```

- [ ] **Step 9.2: Register the Application class in the manifest**

Edit `android-app/app/src/main/AndroidManifest.xml`: change
```xml
    <application
        android:label="@string/app_name"
```
to
```xml
    <application
        android:name=".FtrApplication"
        android:label="@string/app_name"
```

- [ ] **Step 9.3: Create `ui/theme/Color.kt`**

```kotlin
package com.filetransfer.ftr.ui.theme

import androidx.compose.ui.graphics.Color

val Purple80 = Color(0xFFD0BCFF)
val Purple40 = Color(0xFF6650a4)
```

- [ ] **Step 9.4: Create `ui/theme/Type.kt`**

```kotlin
package com.filetransfer.ftr.ui.theme

import androidx.compose.material3.Typography

val AppTypography = Typography()
```

- [ ] **Step 9.5: Create `ui/theme/Theme.kt`**

```kotlin
package com.filetransfer.ftr.ui.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable

private val LightColors = lightColorScheme(primary = Purple40)
private val DarkColors = darkColorScheme(primary = Purple80)

@Composable
fun FtrTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    content: @Composable () -> Unit
) {
    val colors = if (darkTheme) DarkColors else LightColors
    MaterialTheme(colorScheme = colors, typography = AppTypography, content = content)
}
```

- [ ] **Step 9.6: Create `ui/MainScreen.kt` with placeholder tab bodies**

```kotlin
package com.filetransfer.ftr.ui

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowDownward
import androidx.compose.material.icons.filled.ArrowUpward
import androidx.compose.material3.Icon
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Tab
import androidx.compose.material3.TabRow
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier

@Composable
fun MainScreen() {
    var selected by remember { mutableIntStateOf(0) }
    Scaffold(
        topBar = {
            TabRow(selectedTabIndex = selected) {
                Tab(
                    selected = selected == 0,
                    onClick = { selected = 0 },
                    text = { Text("Send") },
                    icon = { Icon(Icons.Default.ArrowUpward, contentDescription = null) }
                )
                Tab(
                    selected = selected == 1,
                    onClick = { selected = 1 },
                    text = { Text("Receive") },
                    icon = { Icon(Icons.Default.ArrowDownward, contentDescription = null) }
                )
            }
        }
    ) { inner ->
        Box(Modifier.fillMaxSize().padding(inner), contentAlignment = Alignment.Center) {
            when (selected) {
                0 -> Text("Send (Task 11)")
                1 -> Text("Receive (Task 10)")
            }
        }
    }
}
```

- [ ] **Step 9.7: Update `MainActivity.kt` to host `MainScreen` under `FtrTheme`**

Replace the contents of `android-app/app/src/main/java/com/filetransfer/ftr/MainActivity.kt`:

```kotlin
package com.filetransfer.ftr

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import com.filetransfer.ftr.ui.MainScreen
import com.filetransfer.ftr.ui.theme.FtrTheme

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            FtrTheme {
                MainScreen()
            }
        }
    }
}
```

- [ ] **Step 9.8: Build**

Run: `cd android-app && ./gradlew assembleDebug`
Expected: `BUILD SUCCESSFUL`.

- [ ] **Step 9.9: Install on a connected device or emulator and visually verify the tabs**

Run: `cd android-app && ./gradlew installDebug && adb shell am start -n com.filetransfer.ftr/.MainActivity`
Expected: two tabs at the top of the screen — "Send" and "Receive". Tapping each swaps placeholder text. If `adb` reports no devices, start an AVD or connect a phone with USB debugging.

- [ ] **Step 9.10: Commit**

```
git add android-app/app/src/main/java/com/filetransfer/ftr/FtrApplication.kt \
        android-app/app/src/main/java/com/filetransfer/ftr/MainActivity.kt \
        android-app/app/src/main/java/com/filetransfer/ftr/ui/theme \
        android-app/app/src/main/java/com/filetransfer/ftr/ui/MainScreen.kt \
        android-app/app/src/main/AndroidManifest.xml
git commit -m "feat(android): add FtrApplication, theme, and MainScreen tabs"
```

---

## Task 10: Receive tab — ViewModel and screen

**Goal:** A working Receive tab that lets the user set name/port/passkey, tap "Join Network", see the receiver status + received-files list, and "Leave Network". Starts/stops `FileReceiver` + `NsdAdvertiser`.

**Files:**
- Create: `android-app/app/src/main/java/com/filetransfer/ftr/ui/receive/ReceiverViewModel.kt`
- Create: `android-app/app/src/main/java/com/filetransfer/ftr/ui/receive/ReceiverScreen.kt`
- Modify: `android-app/app/src/main/java/com/filetransfer/ftr/ui/MainScreen.kt` (replace "Receive" placeholder)

- [ ] **Step 10.1: Create `ReceiverViewModel.kt`**

```kotlin
package com.filetransfer.ftr.ui.receive

import android.app.Application
import android.os.Build
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.filetransfer.ftr.FtrApplication
import com.filetransfer.ftr.net.FileReceiver
import com.filetransfer.ftr.net.NsdAdvertiser
import com.filetransfer.ftr.util.PassKey
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

class ReceiverViewModel(app: Application) : AndroidViewModel(app) {

    private val ftr: FtrApplication = app as FtrApplication
    private val receiver: FileReceiver = ftr.fileReceiver
    private val advertiser: NsdAdvertiser = ftr.nsdAdvertiser

    val isRunning: StateFlow<Boolean> = receiver.isRunning
    val receivedFiles: StateFlow<List<String>> = receiver.receivedFiles

    private val _name = MutableStateFlow(defaultName())
    val name: StateFlow<String> = _name.asStateFlow()

    private val _port = MutableStateFlow(8844)
    val port: StateFlow<Int> = _port.asStateFlow()

    private val _passKey = MutableStateFlow(PassKey.random(6))
    val passKey: StateFlow<String> = _passKey.asStateFlow()

    private val _error = MutableStateFlow<String?>(null)
    val error: StateFlow<String?> = _error.asStateFlow()

    fun setName(v: String) { _name.value = v }
    fun setPort(v: Int) { _port.value = v }
    fun setPassKey(v: String) { _passKey.value = v }

    fun start() {
        _error.value = null
        viewModelScope.launch {
            try {
                receiver.start(_port.value, _passKey.value)
                advertiser.register(_name.value, _port.value, receiver.dropDir.absolutePath)
            } catch (e: Exception) {
                _error.value = e.message
                runCatching { receiver.stop() }
                runCatching { advertiser.stop() }
            }
        }
    }

    fun stop() {
        advertiser.stop()
        receiver.stop()
    }

    private fun defaultName(): String =
        Build.MODEL.trim().replace('.', '-').ifBlank { "android" }
}
```

- [ ] **Step 10.2: Create `ReceiverScreen.kt`**

```kotlin
package com.filetransfer.ftr.ui.receive

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel

@Composable
fun ReceiverScreen(vm: ReceiverViewModel = viewModel()) {
    val name by vm.name.collectAsState()
    val port by vm.port.collectAsState()
    val passKey by vm.passKey.collectAsState()
    val isRunning by vm.isRunning.collectAsState()
    val error by vm.error.collectAsState()
    val received by vm.receivedFiles.collectAsState()

    Column(
        modifier = Modifier.fillMaxSize().padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        OutlinedTextField(
            value = name,
            onValueChange = vm::setName,
            label = { Text("Device name") },
            enabled = !isRunning,
            modifier = Modifier.fillMaxWidth()
        )
        OutlinedTextField(
            value = port.toString(),
            onValueChange = { s -> s.toIntOrNull()?.let(vm::setPort) },
            label = { Text("Port") },
            enabled = !isRunning,
            modifier = Modifier.fillMaxWidth()
        )
        OutlinedTextField(
            value = passKey,
            onValueChange = vm::setPassKey,
            label = { Text("Passkey") },
            enabled = !isRunning,
            modifier = Modifier.fillMaxWidth()
        )

        Button(
            onClick = { if (isRunning) vm.stop() else vm.start() },
            modifier = Modifier.fillMaxWidth()
        ) {
            Text(if (isRunning) "Leave Network" else "Join Network")
        }

        error?.let { Text(it, color = Color.Red) }

        if (isRunning) {
            Text("Listening on port $port. Others can send with:")
            Text("ftr send --key $passKey <file> $name")
        }

        HorizontalDivider()
        Text("Received files")
        LazyColumn(modifier = Modifier.fillMaxWidth()) {
            items(received) { file -> Text(file) }
        }
    }
}
```

- [ ] **Step 10.3: Wire `ReceiverScreen` into `MainScreen`**

Edit `android-app/app/src/main/java/com/filetransfer/ftr/ui/MainScreen.kt`: replace the line
```kotlin
                1 -> Text("Receive (Task 10)")
```
with
```kotlin
                1 -> com.filetransfer.ftr.ui.receive.ReceiverScreen()
```

- [ ] **Step 10.4: Build and install**

Run:
```
cd android-app && ./gradlew installDebug && adb shell am start -n com.filetransfer.ftr/.MainActivity
```
Expected: the Receive tab now shows the form and "Join Network" button. Tapping it starts the server — confirm by running `ftr list` on a laptop on the same Wi-Fi; the Android device should appear.

- [ ] **Step 10.5: Commit**

```
git add android-app/app/src/main/java/com/filetransfer/ftr/ui/receive \
        android-app/app/src/main/java/com/filetransfer/ftr/ui/MainScreen.kt
git commit -m "feat(android): add Receive tab with ViewModel, start/stop, status UI"
```

---

## Task 11: Send tab — ViewModel and screen with SAF pickers

**Goal:** Peer list from `NsdBrowser`, per-peer "Send File" / "Send Folder" buttons, passkey prompt, SAF file/tree picker, send via `FileSender`, surface success/error state.

**Files:**
- Create: `android-app/app/src/main/java/com/filetransfer/ftr/ui/send/PeerListViewModel.kt`
- Create: `android-app/app/src/main/java/com/filetransfer/ftr/ui/send/PeerListScreen.kt`
- Modify: `android-app/app/src/main/java/com/filetransfer/ftr/ui/MainScreen.kt` (replace "Send" placeholder)

- [ ] **Step 11.1: Create `PeerListViewModel.kt`**

```kotlin
package com.filetransfer.ftr.ui.send

import android.app.Application
import android.net.Uri
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.filetransfer.ftr.FtrApplication
import com.filetransfer.ftr.model.Peer
import com.filetransfer.ftr.net.FileSender
import com.filetransfer.ftr.net.NsdBrowser
import io.ktor.http.HttpStatusCode
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

sealed class SendState {
    object Idle : SendState()
    object Sending : SendState()
    data class Success(val message: String) : SendState()
    data class Error(val message: String) : SendState()
}

class PeerListViewModel(app: Application) : AndroidViewModel(app) {

    private val ftr: FtrApplication = app as FtrApplication
    private val browser: NsdBrowser = ftr.nsdBrowser
    private val sender: FileSender = ftr.fileSender

    val peers: StateFlow<List<Peer>> = browser.peers
    val isSearching: StateFlow<Boolean> = browser.isSearching

    private val _sendState = MutableStateFlow<SendState>(SendState.Idle)
    val sendState: StateFlow<SendState> = _sendState.asStateFlow()

    fun startBrowsing() { browser.start() }
    fun stopBrowsing() { browser.stop() }

    fun sendFile(uri: Uri, peer: Peer, passKey: String) {
        _sendState.value = SendState.Sending
        viewModelScope.launch {
            val result = sender.sendFile(uri, peer, passKey)
            _sendState.value = interpret(result, peer.name)
        }
    }

    fun sendDirectory(treeUri: Uri, peer: Peer, passKey: String) {
        _sendState.value = SendState.Sending
        viewModelScope.launch {
            val result = sender.sendDirectory(treeUri, peer, passKey)
            _sendState.value = interpret(result, peer.name)
        }
    }

    private fun interpret(result: Result<HttpStatusCode>, peerName: String): SendState =
        result.fold(
            onSuccess = { code ->
                when (code) {
                    HttpStatusCode.OK -> SendState.Success("Sent to $peerName")
                    HttpStatusCode.Unauthorized -> SendState.Error("Invalid passkey")
                    HttpStatusCode.Conflict -> SendState.Error("File already exists on $peerName")
                    else -> SendState.Error("Server returned ${code.value}")
                }
            },
            onFailure = { e -> SendState.Error("Could not reach $peerName: ${e.message}") }
        )
}
```

- [ ] **Step 11.2: Create `PeerListScreen.kt`**

```kotlin
package com.filetransfer.ftr.ui.send

import android.net.Uri
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import com.filetransfer.ftr.model.Peer

@Composable
fun PeerListScreen(vm: PeerListViewModel = viewModel()) {
    val peers by vm.peers.collectAsState()
    val isSearching by vm.isSearching.collectAsState()
    val sendState by vm.sendState.collectAsState()

    LaunchedEffect(Unit) { vm.startBrowsing() }

    var chosenPeer by remember { mutableStateOf<Peer?>(null) }
    var passKey by remember { mutableStateOf("") }
    var mode by remember { mutableStateOf<Mode?>(null) }  // FILE or FOLDER

    val filePicker = rememberLauncherForActivityResult(
        ActivityResultContracts.OpenDocument()
    ) { uri: Uri? ->
        val peer = chosenPeer
        if (uri != null && peer != null) vm.sendFile(uri, peer, passKey)
        reset(chosenPeer = null, passKey = "", mode = null) { p, k, m ->
            chosenPeer = p; passKey = k; mode = m
        }
    }
    val folderPicker = rememberLauncherForActivityResult(
        ActivityResultContracts.OpenDocumentTree()
    ) { uri: Uri? ->
        val peer = chosenPeer
        if (uri != null && peer != null) vm.sendDirectory(uri, peer, passKey)
        reset(chosenPeer = null, passKey = "", mode = null) { p, k, m ->
            chosenPeer = p; passKey = k; mode = m
        }
    }

    Column(Modifier.fillMaxSize().padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text("Peers", modifier = Modifier.weight(1f))
            if (isSearching) CircularProgressIndicator()
        }
        HorizontalDivider()

        if (peers.isEmpty()) {
            Box(Modifier.fillMaxWidth().padding(vertical = 32.dp), contentAlignment = Alignment.Center) {
                Text(if (isSearching) "Searching…" else "No peers found")
            }
        } else {
            LazyColumn {
                items(peers, key = { it.id }) { peer ->
                    Column(
                        Modifier.fillMaxWidth().clickable { chosenPeer = peer }.padding(vertical = 8.dp)
                    ) {
                        Text(peer.name)
                        Text("${peer.host}:${peer.port}", color = Color.Gray)
                    }
                }
            }
        }

        when (val s = sendState) {
            SendState.Sending -> Text("Sending…")
            is SendState.Success -> Text(s.message, color = Color(0xFF2E7D32))
            is SendState.Error -> Text(s.message, color = Color.Red)
            SendState.Idle -> Unit
        }
    }

    chosenPeer?.let { peer ->
        AlertDialog(
            onDismissRequest = { chosenPeer = null; passKey = "" },
            title = { Text("Send to ${peer.name}") },
            text = {
                Column {
                    OutlinedTextField(
                        value = passKey,
                        onValueChange = { passKey = it },
                        label = { Text("Passkey") },
                        modifier = Modifier.fillMaxWidth()
                    )
                    Spacer(Modifier.width(12.dp))
                }
            },
            confirmButton = {
                Row {
                    TextButton(onClick = {
                        mode = Mode.FILE
                        filePicker.launch(arrayOf("*/*"))
                    }) { Text("Send File") }
                    TextButton(onClick = {
                        mode = Mode.FOLDER
                        folderPicker.launch(null)
                    }) { Text("Send Folder") }
                }
            },
            dismissButton = {
                TextButton(onClick = { chosenPeer = null; passKey = "" }) { Text("Cancel") }
            }
        )
    }
}

private enum class Mode { FILE, FOLDER }

private inline fun reset(
    chosenPeer: Peer?, passKey: String, mode: Mode?,
    update: (Peer?, String, Mode?) -> Unit
) { update(chosenPeer, passKey, mode) }
```

- [ ] **Step 11.3: Wire `PeerListScreen` into `MainScreen`**

Edit `android-app/app/src/main/java/com/filetransfer/ftr/ui/MainScreen.kt`: replace
```kotlin
                0 -> Text("Send (Task 11)")
```
with
```kotlin
                0 -> com.filetransfer.ftr.ui.send.PeerListScreen()
```

- [ ] **Step 11.4: Build and install**

Run:
```
cd android-app && ./gradlew installDebug && adb shell am start -n com.filetransfer.ftr/.MainActivity
```
Expected: Send tab lists peers discovered on the LAN. If `ftr join` is running on a laptop on the same Wi-Fi, that laptop should appear in the list.

- [ ] **Step 11.5: Commit**

```
git add android-app/app/src/main/java/com/filetransfer/ftr/ui/send \
        android-app/app/src/main/java/com/filetransfer/ftr/ui/MainScreen.kt
git commit -m "feat(android): add Send tab with peer list and SAF file/folder pickers"
```

---

## Task 12: FileProvider, network security config, drop-dir visibility

**Goal:** Expose the received-files directory to Android's Files app, and explicitly allow cleartext HTTP to private IPs so the Ktor client can POST to `http://192.168.x.y/` without hitting the system-wide cleartext block.

**Files:**
- Create: `android-app/app/src/main/res/xml/file_paths.xml`
- Create: `android-app/app/src/main/res/xml/network_security_config.xml`
- Modify: `android-app/app/src/main/AndroidManifest.xml`

- [ ] **Step 12.1: Create `file_paths.xml`**

```xml
<?xml version="1.0" encoding="utf-8"?>
<paths xmlns:android="http://schemas.android.com/apk/res/android">
    <external-files-path name="drop" path="drop/" />
</paths>
```

- [ ] **Step 12.2: Create `network_security_config.xml`**

```xml
<?xml version="1.0" encoding="utf-8"?>
<network-security-config>
    <base-config cleartextTrafficPermitted="false" />
    <domain-config cleartextTrafficPermitted="true">
        <domain includeSubdomains="false">10.0.0.0/8</domain>
        <domain includeSubdomains="false">172.16.0.0/12</domain>
        <domain includeSubdomains="false">192.168.0.0/16</domain>
        <domain includeSubdomains="false">169.254.0.0/16</domain>
        <domain includeSubdomains="false">127.0.0.1</domain>
        <domain includeSubdomains="false">localhost</domain>
    </domain-config>
</network-security-config>
```

Note: Android's `<domain>` element technically matches hostnames, not CIDR ranges. Since ftr peers are reached by raw IP, we rely on each address being in one of the RFC1918 ranges. If this proves too restrictive at runtime (e.g., the system picks a public-IP interface), drop back to `android:usesCleartextTraffic="true"` on `<application>` — documented as a known limitation rather than a blocker.

- [ ] **Step 12.3: Extend `AndroidManifest.xml` with the FileProvider and network config**

Change the `<application>` opening tag to include the network security config:

```xml
    <application
        android:name=".FtrApplication"
        android:label="@string/app_name"
        android:theme="@style/Theme.FileTransfer"
        android:allowBackup="true"
        android:supportsRtl="true"
        android:usesCleartextTraffic="true"
        android:networkSecurityConfig="@xml/network_security_config">
```

Add a `<provider>` element inside `<application>`, immediately after the `<activity>` block:

```xml
        <provider
            android:name="androidx.core.content.FileProvider"
            android:authorities="com.filetransfer.ftr.fileprovider"
            android:exported="false"
            android:grantUriPermissions="true">
            <meta-data
                android:name="android.support.FILE_PROVIDER_PATHS"
                android:resource="@xml/file_paths" />
        </provider>
```

- [ ] **Step 12.4: Build and install**

Run: `cd android-app && ./gradlew installDebug`
Expected: `BUILD SUCCESSFUL`.

- [ ] **Step 12.5: Commit**

```
git add android-app/app/src/main/res/xml \
        android-app/app/src/main/AndroidManifest.xml
git commit -m "feat(android): add FileProvider and permissive cleartext for LAN IPs"
```

---

## Task 13: README update + manual interop test matrix

**Goal:** Document the Android app in the project README (how to build, install, test) and record a manual interop-test checklist so humans know what to click through before declaring v1 done.

**Files:**
- Modify: `README.md`

- [ ] **Step 13.1: Append an "Android" section to `README.md`**

Add after the existing `## How It Works` section (or at the bottom of the file):

```markdown
---

## Android app

A native Android client lives in `android-app/`. It speaks the same mDNS + HTTP protocol as the Go CLI and iOS app, so any combination of the three can discover and send to each other on the same LAN.

### Build and install

Requirements: JDK 17+, Android SDK (API 34), `adb` on `$PATH`, a connected device with USB debugging enabled or a running AVD (API 26+).

```bash
cd android-app
./gradlew installDebug
adb shell am start -n com.filetransfer.ftr/.MainActivity
```

To produce a standalone APK for sideloading:

```bash
cd android-app
./gradlew assembleDebug
# Output: android-app/app/build/outputs/apk/debug/app-debug.apk
```

### Testing

Run the JVM unit tests (tar.gz round-trip, zip-slip, TXT record fixture, HTTP client shape, receiver routes):

```bash
cd android-app
./gradlew :app:testDebugUnitTest
```

### Manual interop matrix

Before declaring a change done, run through this checklist with both a laptop and an Android device on the same Wi-Fi:

- [ ] **Go ↔ Android, file:** start `ftr join` on the laptop; on Android tap Send → pick the laptop → pick a small file → verify it lands in the laptop's drop dir.
- [ ] **Android ↔ Go, file:** on Android tap Receive → Join Network; on laptop run `ftr send --key <key> <file> <android-name>`; verify it appears in the Android Files app under `Android/data/com.filetransfer.ftr/files/drop/`.
- [ ] **Go ↔ Android, directory:** same as above but send a folder with at least one subdirectory and one binary file. Verify nested files and a non-empty binary round-trip byte-for-byte.
- [ ] **Android ↔ Go, directory:** same direction, same shape.
- [ ] **iOS ↔ Android, file and directory:** both directions.
- [ ] **Android ↔ Android, file and directory:** between two Android devices.
- [ ] **Passkey mismatch:** confirm a wrong key produces "Invalid passkey" on the sender and the receiver's drop dir stays empty.
- [ ] **Overwrite guard:** send the same filename twice; confirm the second attempt reports "File already exists".
- [ ] **Port in use:** start the receiver twice with the same port; confirm the second attempt shows a visible error.
```

- [ ] **Step 13.2: Commit**

```
git add README.md
git commit -m "docs: document Android app build, install, and interop test matrix"
```

---

## Final smoke check

After Task 13, run one combined verification pass before declaring the plan done:

- [ ] `cd android-app && ./gradlew clean assembleDebug testDebugUnitTest`
  Expected: `BUILD SUCCESSFUL`, all tests pass.
- [ ] Install on a real device, run through at least the two `Go ↔ Android, file` rows of the interop matrix.
- [ ] `git log --oneline main..HEAD` should show roughly 13 commits, one per task.

---

## Self-review notes (from writing-plans skill)

**Spec coverage check:** Every section of `docs/superpowers/specs/2026-04-10-android-app-design.md` is addressed by at least one task in this plan:

- Tech stack + minSdk + Gradle setup → Task 1
- Peer model, PassKey util → Task 2
- TarGz format specifics (top-level skip, dir perm fix-up, zip-slip) → Task 3
- TXT record encoding quirk → Task 4 (with Go fixture)
- NsdBrowser with MulticastLock → Task 5
- FileReceiver route semantics (401, 409, dir extract) → Task 6
- FileSender single file → Task 7
- FileSender directory tar send → Task 8
- FtrApplication singletons, theme, MainScreen tabs → Task 9
- Receive tab flow (start/stop, status, received list) → Task 10
- Send tab flow (peer list, passkey prompt, SAF pickers) → Task 11
- FileProvider + cleartext network security → Task 12
- README docs + manual interop matrix → Task 13

**Spec items intentionally deferred (matching the spec's v1 non-goals):** foreground-service receiver, instrumentation/UI tests, CI wiring, MediaStore/SAF-picked drop dir, resumable transfers, progress bar. None of these are task gaps — they are explicit v1 non-goals.

**Spec open questions left to implementation time:**
- TXT record encoding — Task 4's fixture test resolves this definitively: if the assertion fails, try `setAttribute(dropDir, "")` instead of `setAttribute(dropDir, null)`.
- `CHANGE_WIFI_MULTICAST_STATE` necessity on API 26+ — the permission is kept in Task 1's manifest because it is harmless if unused and avoids a "works on my device but not yours" surprise.
