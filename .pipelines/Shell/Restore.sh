#!/bin/bash
# https://ev2docs.azure.net/features/service-artifacts/actions/shell-extensions/overview.html#script-authoring-and-packaging
# https://msazure.visualstudio.com/Azure-Express/_git/Samples?path=%2FServiceGroupRoot&version=GBmaster
# Debugging information
echo "Current directory: $(pwd)"
ls -l

# package the script for ev2 shell extension
tar -C ./Shell -cvf Run.tar ./Run.sh

# write the ev2 version file
echo -n $BUILD_BUILDNUMBER | tee ./EV2Specs/BuildVer.txt

ARCHS=("amd64" "arm64")

for arch in "${ARCHS[@]}"; do
    mkdir -p "$arch"

    IMAGE_NAMES=("agent" "init" "operator" "kubectl" "shell")

    for image in "${IMAGE_NAMES[@]}"; do

        ORIGINAL_DIRECTORY="../../../retina-oss-build/drop_build_${image}_linux_${arch}_ImageBuild"

        if [ ! -d "$ORIGINAL_DIRECTORY" ]; then
            echo "Error: Directory does not exist - $ORIGINAL_DIRECTORY"
            continue
        fi

        for file in "$ORIGINAL_DIRECTORY"/*; do
            echo "Processing file: $file"
            if [[ "$file" == *.tar.gz ]]; then
                echo "Decompressing file: $file"
                gunzip "$file" && echo "Decompressed: $file"
                file="${file%.gz}"
                if [ -f "$file" ]; then
                    echo "Moving file: $file to ./$arch/"
                    mv "$file" "./$arch/"
                else
                    echo "Error: File $file not found after decompression."
                fi
            else
                echo "Skipping non-matching file: $file"
            fi
        done

        echo "$arch Folder Contents:"
        ls -alF "$arch"
    done
done


# package all the pipeline output image to a tar file
mkdir multi_arch_image

# move all the image to the multi_arch_image folder
mv ./amd64/* ./multi_arch_image/
mv ./arm64/* ./multi_arch_image/

# package the folder output to tar since the rollout parameter only accept a specifc file
tar -cvf multi_arch_image.tar ./multi_arch_image
