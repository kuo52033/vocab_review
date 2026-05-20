# iOS app

This SwiftUI shell covers:

- development magic-link sign-in against the Go backend, with manual token fallback
- one-card-at-a-time due review flow
- review grading actions with answer reveal
- notification permission request

Open the folder in Xcode to generate the `.xcodeproj` settings and run it with your local backend.

Release configuration is controlled by `VocabReview/Config/Release.xcconfig` and points beta builds at `https://api.vocabreview.uk`.

For production beta testing from Xcode, select the shared `VocabReview Beta` scheme. The default Debug run configuration points at `http://localhost:8080`, so it requires the local backend to be running and only works from the simulator, not a physical iPhone.
