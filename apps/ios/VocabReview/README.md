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

Debug/local runs require the local backend to be running and only work from the simulator unless your physical device can reach your Mac's backend address.
