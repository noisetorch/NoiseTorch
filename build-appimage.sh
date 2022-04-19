#!/bin/sh
#
# Unforuntately we can't use regular AppImage tooling to build an AppImage.
# The reason for that is that we require CAP_SYS_RESOURCE, which doesn't
# work from FUSE mounted SquashFS, because it's mounted with 'nosuid'.
# Interestingly, it seem the process is also not allowed to inherit
# CAP_SYS_RESOURCE while the mount has nosuid. Our only option is to
# make what appimage calls the "runtime", the thing thats supposed to
# mount the embedded SquashFS, our application itself. And use the SquashFS
# only for carrying around metadata, like the icon and .desktop file.
# This means you cannot unpack and run this AppImage, to do that,
# we'd have to include a copy of noisetorch *again*, basically doubling the
# size. Doesn't seem worth it, let's break spec here.
#

WORK=`mktemp -d`
OUTNAME="NoiseTorch-x86_64.AppImage"

go generate

#create mostly empty object file
touch $WORK/empty.c

gcc -c $WORK/empty.c -o $WORK/empty.o
rm $WORK/empty.c

#insert the sections we want into the object with objcopy
#echo "zsync-something|justatest" > $WORK/upd_info
printf "\0" > $WORK/upd_info
objcopy  --add-section .upd_info=$WORK/upd_info --set-section-flags .upd_info=noload,readonly $WORK/empty.o #add section with objcopy
rm $WORK/upd_info

#echo "sig key goes here" > $WORK/sig_key
printf "\0" > $WORK/sig_key
objcopy  --add-section .sig_key=$WORK/sig_key --set-section-flags .sig_key=noload,readonly $WORK/empty.o
rm $WORK/sig_key

#echo "hash_placeholder_must_be_filed_after_this_should_be_nullbytes" > $WORK/sha256
printf "\0" > $WORK/sha256
objcopy  --add-section .sha256_sig=$WORK/sha256 --set-section-flags .sha256_sig=noload,readonly $WORK/empty.o #add section with objcopy
rm $WORK/sha256


#i'm sure there's a way to get an *actually* empty object file first.
#or a better way in general to get an object with our sections
#but.... this kinda works for now, so....
objcopy --remove-section=.comment $WORK/empty.o
objcopy --remove-section=.data $WORK/empty.o
objcopy --remove-section=.text $WORK/empty.o
objcopy --remove-section=.bss $WORK/empty.o
objcopy --remove-section=.note.GNU-stack $WORK/empty.o
objcopy --remove-section=.note.gnu.property $WORK/empty.o
objdump -x $WORK/empty.o



#link the created object into our app so we get the sections
#why musl? because, as far as i understand, if we set linkmode=external, then go
#doesn't include it's startup code, e.g _rt0_amd64 but instead expects _start to do
#c-style init. But if we link with -nostdlib then we don't get _start and you can't run the app.
#but if we do have the c stdlib, it's glibc for most people which can't be statically linked.
#so, we need to link with musl to get a static binary and be able to include our object here.
CC=musl-gcc go build -o "$OUTNAME" -ldflags="-linkmode=external -extldflags \"$WORK/empty.o -static\" -X main.version=${VERSION} -X main.distribution=official-appimage"
strip "$OUTNAME"
rm $WORK/empty.o


#write app image 2 magic bytes
printf "AI\02" | dd of="$OUTNAME" bs=1 count=3 seek=8 conv=notrunc

#append squashfs with metadata
mkdir $WORK/sqfs

# the second copy of ourself INSIDE the app image, that is never used if launched via the "runtime"
CGO_ENABLED=0 GOOS=linux go build -o $WORK/sqfs/AppRun -trimpath -tags release -a -ldflags "-s -w -extldflags \"-static\" -X main.version=${VERSION} -X main.distribution=official-appimage-inner" .
# echo "#!/bin/sh" > $WORK/sqfs/AppRun
# echo "echo 'This application cannot be ran in unpacked AppDir mode.'" >> $WORK/sqfs/AppRun
# echo "echo 'Sorry about that.'" >> $WORK/sqfs/AppRun
# echo "echo 'If you wonder why, it is because hacks were required to make it work at all.'" >> $WORK/sqfs/AppRun
chmod +x $WORK/sqfs/AppRun

cp assets/noisetorch.desktop $WORK/sqfs/
echo "X-AppImage-Version=$(git describe --tags)">>$WORK/sqfs/noisetorch.desktop
cp assets/icon/noisetorch.png $WORK/sqfs/.DirIcon
mksquashfs $WORK/sqfs/* $WORK/sqfs/.* $WORK/out.sqfs

rm $WORK/sqfs/.DirIcon
rm $WORK/sqfs/noisetorch.desktop
rm $WORK/sqfs/AppRun
rmdir $WORK/sqfs

cat $WORK/out.sqfs >> "$OUTNAME"

rm $WORK/out.sqfs


rmdir $WORK
