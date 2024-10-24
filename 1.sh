#!/bin/bash

# soft-deleted 객체들을 리스트로 가져옵니다.
OBJECTS="$(gcloud storage ls gs://sg-prod-stepin-bucket/video_hls --soft-deleted --recursive)"
cat $OBJECTS > video_hls.log
# 객체 리스트에서 각 객체와 generation 번호를 추출하여 복원합니다.
echo "$OBJECTS" | while read -r line; do
    # 객체 경로와 generation 번호가 포함된 줄만 필터링합니다.
    if [[ $line =~ ^(gs://[^#]+)#([0-9]+)$ ]]; then
        OBJECT_PATH="${BASH_REMATCH[1]}"
        GENERATION="${BASH_REMATCH[2]}"

        # 복원 명령어 실행
        echo "Restoring: ${OBJECT_PATH} (Generation: ${GENERATION})"
        gcloud storage restore "${OBJECT_PATH}"/"${GENERATION}"
    fi
done