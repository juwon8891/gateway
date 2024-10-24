#!/bin/bash

# 버킷 경로와 기본 경로 설정
BUCKET="gs://sg-prod-stepin-bucket"
BASE_PATH="video_hls"

# soft-deleted 상태의 객체 리스트를 가져옵니다.
SOFT_DELETED_OBJECTS=$(gcloud storage ls "${BUCKET}/${BASE_PATH}" --soft-deleted --recursive | awk -F'#' '{print $1}')

# 현재 버킷에 존재하는 모든 객체 리스트를 가져옵니다.
CURRENT_OBJECTS=$(gcloud storage ls "${BUCKET}/${BASE_PATH}" --recursive)

# 복구되지 않은 객체들을 추출합니다.
echo "$SOFT_DELETED_OBJECTS" | while read -r object; do
    if ! echo "$CURRENT_OBJECTS" | grep -q "$object"; then
        echo "복구되지 않은 객체: $object"
    fi
done

echo "검증 완료: 위에 출력된 객체가 없으면 모든 객체가 복구된 것입니다."
