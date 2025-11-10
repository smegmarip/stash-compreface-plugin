#!/usr/bin/env python3
"""
Test script for Quality Service
Tests all endpoints with real images
"""

import os
import sys
import json
import base64
import requests
from pathlib import Path

# Service URL
SERVICE_URL = os.environ.get('QUALITY_SERVICE_URL', 'http://localhost:6001')


def test_health():
    """Test health check endpoint"""
    print("Testing /health endpoint...")
    try:
        response = requests.get(f"{SERVICE_URL}/health")
        response.raise_for_status()
        data = response.json()
        print(f"✓ Health check passed: {data}")
        return True
    except Exception as ex:
        print(f"✗ Health check failed: {ex}")
        return False


def test_assess_quality(image_path):
    """Test quality assessment endpoint"""
    print(f"\nTesting /quality/assess with {image_path}...")

    if not os.path.exists(image_path):
        print(f"✗ Image not found: {image_path}")
        return False

    try:
        # Read image file
        with open(image_path, 'rb') as f:
            image_data = f.read()

        # Test with multipart form data
        files = {'file': ('test.jpg', image_data, 'image/jpeg')}

        # Example face bounding box (you may need to adjust these)
        faces_data = [
            {"box": {"x_min": 50, "y_min": 50, "x_max": 200, "y_max": 200}}
        ]

        data = {'faces': json.dumps(faces_data)}

        response = requests.post(
            f"{SERVICE_URL}/quality/assess",
            files=files,
            data=data
        )
        response.raise_for_status()

        result = response.json()
        print(f"✓ Assessment completed")
        print(f"  Found {len(result.get('faces', []))} face(s)")

        for i, face in enumerate(result.get('faces', [])):
            conf = face.get('confidence')
            if conf:
                print(f"  Face {i+1}:")
                print(f"    Score: {conf.get('score', 'N/A')}")
                print(f"    Type: {conf.get('type', 'N/A')}")
                print(f"    Size: {face.get('cropped_size', 'N/A')}")

        return True

    except Exception as ex:
        print(f"✗ Assessment failed: {ex}")
        import traceback
        traceback.print_exc()
        return False


def test_preprocess(image_path):
    """Test preprocessing endpoint"""
    print(f"\nTesting /quality/preprocess with {image_path}...")

    if not os.path.exists(image_path):
        print(f"✗ Image not found: {image_path}")
        return False

    try:
        with open(image_path, 'rb') as f:
            image_data = f.read()

        files = {'file': ('test.jpg', image_data, 'image/jpeg')}

        response = requests.post(
            f"{SERVICE_URL}/quality/preprocess",
            files=files
        )
        response.raise_for_status()

        # Save result
        output_path = '/tmp/preprocessed_test.jpg'
        with open(output_path, 'wb') as f:
            f.write(response.content)

        print(f"✓ Preprocessing completed")
        print(f"  Output saved to: {output_path}")
        print(f"  Size: {len(response.content)} bytes")

        return True

    except Exception as ex:
        print(f"✗ Preprocessing failed: {ex}")
        import traceback
        traceback.print_exc()
        return False


def test_detect(image_path):
    """Test enhanced detection endpoint"""
    print(f"\nTesting /quality/detect with {image_path}...")

    if not os.path.exists(image_path):
        print(f"✗ Image not found: {image_path}")
        return False

    try:
        with open(image_path, 'rb') as f:
            image_data = f.read()

        files = {'file': ('test.jpg', image_data, 'image/jpeg')}

        response = requests.post(
            f"{SERVICE_URL}/quality/detect",
            files=files
        )
        response.raise_for_status()

        result = response.json()
        print(f"✓ Detection completed")
        print(f"  Found {len(result.get('faces', []))} face(s)")

        for i, face in enumerate(result.get('faces', [])):
            box = face.get('box', {})
            conf = face.get('confidence', {})
            print(f"  Face {i+1}:")
            print(f"    Box: ({box.get('x_min')}, {box.get('y_min')}) to ({box.get('x_max')}, {box.get('y_max')})")
            print(f"    Score: {conf.get('score', 'N/A')}")
            print(f"    Type: {conf.get('type', 'N/A')}")
            print(f"    Size: {face.get('cropped_size', 'N/A')}")

        return True

    except Exception as ex:
        print(f"✗ Detection failed: {ex}")
        import traceback
        traceback.print_exc()
        return False


def main():
    """Run all tests"""
    print("=" * 60)
    print("Quality Service Test Suite")
    print("=" * 60)
    print(f"Service URL: {SERVICE_URL}\n")

    # Test health first
    if not test_health():
        print("\n✗ Service not healthy, aborting tests")
        sys.exit(1)

    # Find test image
    test_image = None
    possible_paths = [
        "/Users/x/dev/resources/repo/SFHQ-dataset/images/SFHQ_sample_4x8.jpg",
        "/Users/x/dev/resources/docker/stash/data/pics/11.jpg",
        "test_image.jpg",
    ]

    for path in possible_paths:
        if os.path.exists(path):
            test_image = path
            break

    if test_image:
        print(f"\nUsing test image: {test_image}")
        test_detect(test_image)
        test_assess_quality(test_image)
        test_preprocess(test_image)
    else:
        print("\n⚠ No test image found, skipping image-based tests")
        print(f"  Searched paths:")
        for path in possible_paths:
            print(f"    - {path}")

    print("\n" + "=" * 60)
    print("Test suite completed")
    print("=" * 60)


if __name__ == '__main__':
    main()
