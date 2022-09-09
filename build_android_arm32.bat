SET GOOS=android
SET GOARCH=arm
SET CGO_ENABLED=1
SET CC=E:\Android_NDK\toolchains-r20\bin\arm-linux-androideabi-gcc
@REM SET CC=E:\Android_NDK\android-ndk-r24\toolchains\llvm\prebuilt\windows-x86_64\bin\armv7a-linux-androideabi19-clang.cmd
go build -buildvcs=false -o output/tts-server-go_arm32

pause