# PicHub 安卓客户端

使用 Android 10 或以上的设备。首次打开时填写 PicHub 服务地址，例如 `https://pichub.example.org`，或直接填写分发地址 `https://pichub.example.org/e/wallpaper`。支持 HTTPS 和解析到私网的 `.local` HTTP 地址，例如 `http://pichub.local:5721`。没有 HTTPS 或可解析的 `.local` 名称时，需要先为服务配置其中一种。调试构建还允许模拟器地址 `http://10.0.2.2:5721` 和回环地址。

App 默认只看图和保存，仅需填写分发链接。需要反馈时，在「设置」打开「启用点赞和点踩」，再填入控制台「设置 → 安全鉴权」生成的安卓 App 密钥。关闭赞踩并保存设置会移除手机中保存的密钥。App 密钥只在生成时显示一次，重新生成会使旧密钥失效。

在 PicHub 控制台「设置 → 本地缓存」启用缓存代理模式，以准确筛选横图、竖图并获得赞踩所需的图片 ID。用「测试连接」检查当前模式下的连接。

点击保存图标或长按图片可保存原图。保存路径默认为 `Pictures/PicHub`，可在设置中通过系统选择器指定其他目录。

图片默认完整显示，也可在设置改为铺满屏幕。横屏铺满时按高度等比缩放。启动时自动显示工具栏。竖屏时上滑展开、下滑收起，横屏时下滑展开、上滑收起，横屏工具栏在右侧。开启「显示详情」后，工具栏和图源、原图及取图信息一起展开。

左滑取下一张，右滑回看一张。回看时第一次左滑返回当前图，继续左滑才取新图。配置 App 密钥后，可在设置选择本机的取图标签。设置和标签选择都在返回时保存。筛选对主接口和自定义接口都可用，只影响此 App 的取图请求。横屏和竖屏可同时勾选，两种图片都能返回。只选一种时按该标签取图。只选自适应时按设备方向取图。清空筛选即沿用当前接口的服务端范围。再次点击已选赞踩可撤销，使用统计的分发历史会显示对应请求的 App 反馈。Tag 选择、撤销和历史同步需要支持这些功能的服务端。

## 开发

需要 JDK 17 或 21、Android SDK Platform 36、Build Tools 35.0.0。在 Android Studio 中打开本目录，或从本目录执行：

```sh
bash ./gradlew :app:testDebugUnitTest :app:lintDebug :app:assembleDebug
```

Windows 使用 `gradlew.bat`。调试 APK 在 `app/build/outputs/apk/debug/app-debug.apk`，包名带 `.debug`，可与正式版共存。

## 随主项目发布

版本标签触发主项目 Release。Android 版本名使用 Git 标签，版本号使用该发布工作流的递增运行编号。正式构建会用 R8 精简代码和资源，并压缩 APK 中的 DEX。GitHub Actions 验证签名和包体积后，将不超过 5 MiB 的通用包 `pichub-android.apk` 放在同一 Release 下。通用包含 ARM64、32 位 ARM、x86_64 和 x86。

需在仓库 Actions Secrets 中保存：

- `PICHUB_ANDROID_KEYSTORE_BASE64`（正式签名文件的 Base64）
- `PICHUB_ANDROID_KEYSTORE_PASSWORD`
- `PICHUB_ANDROID_KEY_ALIAS`
- `PICHUB_ANDROID_KEY_PASSWORD`

每次发布沿用同一正式签名密钥，已安装用户才能原位升级。未配置签名时，工作流仍会验证安卓构建，Release 继续发布现有服务端附件。
