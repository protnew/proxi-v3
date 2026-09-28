# P28 — webrtc pin / verification-metadata
- File: prototypes/android/gradle/verification-metadata.xml
- Gradle can parse the scaffold (erify-metadata=true, empty components OK until pin).
- Full pin only after X5 artifact decision — **no Choser card created in this increment**.
- Check: xml well-formed; optional gradle --write-verification-metadata later.