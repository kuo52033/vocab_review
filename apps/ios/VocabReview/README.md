# iOS app

This SwiftUI shell covers:

- development magic-link sign-in against the Go backend, with manual token fallback
- one-card-at-a-time due review flow
- review grading actions with answer reveal
- notification permission request

Open the folder in Xcode to generate the `.xcodeproj` settings and run it with your local backend.

The shared `VocabReview` scheme uses two environments:

- Run uses the Debug configuration from `VocabReview/Config/Debug.xcconfig` and points at `http://localhost:8080`.
- Archive uses the Release configuration from `VocabReview/Config/Release.xcconfig` and points at `https://api.vocabreview.uk`.

Debug/local simulator runs use `localhost`, which means the Mac when running in the simulator. On a physical iPhone, `localhost` means the phone itself, so the app must use your Mac's LAN IP address instead.

For physical-device testing:

1. Make sure the Mac and iPhone are on the same Wi-Fi network.
2. Start the backend on the Mac with `make backend-run`.
3. Find the Mac's LAN IP address, for example `ifconfig | rg "inet "`.
4. Copy `VocabReview/Config/Debug.local.example.xcconfig` to `VocabReview/Config/Debug.local.xcconfig`.
5. Set `VOCAB_REVIEW_API_BASE_URL` in `Debug.local.xcconfig` to `http://<mac-lan-ip>:8080`.
6. Rebuild the app on the phone. iOS may ask for local network permission; allow it.

`Debug.local.xcconfig` is ignored by git because the LAN IP is machine-specific.
