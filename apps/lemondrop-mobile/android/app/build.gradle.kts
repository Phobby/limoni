plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "com.limoni.lemondrop"
    compileSdk = 35

    defaultConfig {
        applicationId = "com.limoni.lemondrop"
        minSdk = 24
        targetSdk = 35
        versionCode = 1
        versionName = "1.0.0"
        ndk { abiFilters += listOf("arm64-v8a", "armeabi-v7a", "x86_64") }
    }

    signingConfigs {
        // Release signing reads a keystore outside the repo (docs/BUILD.md).
        create("release") {
            val ks = System.getenv("LEMONDROP_KEYSTORE")
            if (ks != null) {
                storeFile = file(ks)
                storePassword = System.getenv("LEMONDROP_KEYSTORE_PASSWORD")
                keyAlias = System.getenv("LEMONDROP_KEY_ALIAS")
                keyPassword = System.getenv("LEMONDROP_KEY_PASSWORD")
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            if (System.getenv("LEMONDROP_KEYSTORE") != null) signingConfig = signingConfigs.getByName("release")
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions { jvmTarget = "17" }
}

dependencies {
    // The Go game: built by `scripts/build-android.sh` (ebitenmobile bind).
    implementation(files("libs/lemondrop.aar"))
    implementation("androidx.appcompat:appcompat:1.7.0")
    implementation("androidx.core:core-ktx:1.15.0")
}
