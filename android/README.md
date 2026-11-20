# PicHub 安卓客户端

使用 Android 10 或以上的设备。首次打开时填写 PicHub 服务地址，例如 `https://pichub.example.org`，或直接填写分发地址 `https://pichub.example.org/e/wallpaper`。支持 HTTPS 和解析到私网的 `.local` HTTP 地址，例如 `http://pichub.local:5721`。没有 HTTPS 或可解析的 `.local` 名称时，需要先为服务配置其中一种。调试构建还允许模拟器地址 `http://10.0.2.2:5721` 和回环地址。

App 默认只看图和保存，仅需填写分发链接。需要反馈时，在「设置」打开「启用点赞和点踩」，再填入控制台「设置 → 安全鉴权」生成的安卓 App 密钥。关闭赞踩并保存设置会移除手机中保存的密钥。App 密钥只在生成时显示一次，重新生成会使旧密钥失效。

在 PicHub 控制台「设置 → 本地缓存」启用缓存代理模式，以准确筛选横图、竖图并获得赞踩所需的图片 ID。用「测试连接」检查当前模式下的连接。

「保存」把当前图片的原文件写到系统相册 `Pictures/PicHub`。赞踩会修改服务端图源权重，操作完成后保留当前图片。

## 开发

需要 JDK 17 或 21、Android SDK Platform 36、Build Tools 35.0.0。在 Android Studio 中打开本目录，或从本目录执行：

```sh
bash ./gradlew :app:testDebugUnitTest :app:lintDebug :app:assembleDebug
```

Windows 使用 `gradlew.bat`。调试 APK 在 `app/build/outputs/apk/debug/app-debug.apk`，包名带 `.debug`，可与正式版共存。

## 随主项目发布

版本标签触发主项目 Release。Android 版本名使用 Git 标签，版本号使用该发布工作流的递增运行编号。GitHub Actions 完成构建并验证签名后，将 `pichub-android.apk` 放在同一 Release 下。

需在仓库 Actions Secrets 中保存：

- `PICHUB_ANDROID_KEYSTORE_BASE64`（正式签名文件的 Base64）
- `PICHUB_ANDROID_KEYSTORE_PASSWORD`
- `PICHUB_ANDROID_KEY_ALIAS`
- `PICHUB_ANDROID_KEY_PASSWORD`

每次发布沿用同一正式签名密钥，已安装用户才能原位升级。未配置签名时，工作流仍会验证安卓构建，Release 继续发布现有服务端附件。
