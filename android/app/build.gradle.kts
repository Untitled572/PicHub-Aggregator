plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.compose")
}

val signingFile = providers.environmentVariable("PICHUB_ANDROID_KEYSTORE_FILE").orNull

android {
    namespace = "io.github.untitled572.pichub.android"
    compileSdk = 36

    defaultConfig {
        applicationId = "io.github.untitled572.pichub.android"
        minSdk = 29
        targetSdk = 36
        versionCode = providers.gradleProperty("pichubVersionCode").orNull?.toInt() ?: 1
        versionName = providers.gradleProperty("pichubVersionName").orNull ?: "dev"
    }

    if (signingFile != null) {
        signingConfigs.create("release") {
            storeFile = file(signingFile)
            storePassword = providers.environmentVariable("PICHUB_ANDROID_KEYSTORE_PASSWORD").get()
            keyAlias = providers.environmentVariable("PICHUB_ANDROID_KEY_ALIAS").get()
            keyPassword = providers.environmentVariable("PICHUB_ANDROID_KEY_PASSWORD").get()
        }
    }

    buildTypes {
        debug {
            applicationIdSuffix = ".debug"
            versionNameSuffix = "-debug"
        }
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"))
            if (signingFile != null) signingConfig = signingConfigs.getByName("release")
        }
    }

    buildFeatures {
        compose = true
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}

androidComponents {
    onVariants(selector().withBuildType("release")) { variant ->
        variant.packaging.dex.useLegacyPackaging.set(true)
    }
}

kotlin {
    compilerOptions {
        jvmTarget = org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17
    }
}

dependencies {
    implementation(platform("androidx.compose:compose-bom:2025.10.01"))
    implementation("androidx.activity:activity-compose:1.11.0")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.material:material-icons-core:1.7.8")
    implementation("androidx.compose.ui:ui")
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.9.4")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")

    testImplementation("junit:junit:4.13.2")
}
