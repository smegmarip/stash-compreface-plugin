"""
Quality Service for Compreface Plugin
Provides face quality assessment, preprocessing, and enhanced detection
Ports dlib/OpenCV logic from original Python plugin
"""

import os
import json
import uuid
import traceback
from io import BytesIO
from typing import List, Dict, Tuple, Optional

import cv2
import dlib
import numpy as np
from flask import Flask, request, jsonify
from PIL import Image
from werkzeug.utils import secure_filename

# Initialize Flask app
app = Flask(__name__)
app.config['MAX_CONTENT_LENGTH'] = 32 * 1024 * 1024  # 32MB max file size

# Configuration
TEMP_DIR = os.environ.get('QUALITY_SERVICE_TEMP', '/tmp/quality_service')
MIN_PADDING = 25

# Ensure temp directory exists
os.makedirs(TEMP_DIR, exist_ok=True)

# Load dlib models
script_dir = os.path.dirname(os.path.abspath(__file__))
parent_dir = os.path.dirname(script_dir)

# Try to load from quality_service dir first, then parent dir
landmarks_paths = [
    os.path.join(script_dir, "shape_predictor_68_face_landmarks.dat"),
    os.path.join(parent_dir, "shape_predictor_68_face_landmarks.dat"),
]
resnet_paths = [
    os.path.join(script_dir, "dlib_face_recognition_resnet_model_v1.dat"),
    os.path.join(parent_dir, "dlib_face_recognition_resnet_model_v1.dat"),
]

landmarks_file = next((p for p in landmarks_paths if os.path.exists(p)), None)
resnet_file = next((p for p in resnet_paths if os.path.exists(p)), None)

if not landmarks_file or not resnet_file:
    raise RuntimeError(
        "Required dlib model files not found. Please ensure these files exist:\n"
        "  - shape_predictor_68_face_landmarks.dat\n"
        "  - dlib_face_recognition_resnet_model_v1.dat"
    )

# Initialize dlib models
detector = dlib.get_frontal_face_detector()
predictor = dlib.shape_predictor(landmarks_file)
resnet_model = dlib.face_recognition_model_v1(resnet_file)

# DLIB sub-detector types
DLIB_SUBD = ["front", "left", "right", "front-rotate-left", "front-rotate-right", "n/a", "n/a"]


def calculate_iou(box1: Dict, box2: Dict) -> float:
    """Calculate Intersection over Union (IoU) between two bounding boxes"""
    x_left = max(box1["x_min"], box2["x_min"])
    y_top = max(box1["y_min"], box2["y_min"])
    x_right = min(box1["x_max"], box2["x_max"])
    y_bottom = min(box1["y_max"], box2["y_max"])

    if x_right < x_left or y_bottom < y_top:
        return 0.0

    overlap_area = (x_right - x_left) * (y_bottom - y_top)
    area_bbox1 = (box1["x_max"] - box1["x_min"]) * (box1["y_max"] - box1["y_min"])
    area_bbox2 = (box2["x_max"] - box2["x_min"]) * (box2["y_max"] - box2["y_min"])
    total_area = area_bbox1 + area_bbox2 - overlap_area

    return overlap_area / total_area


def find_best_matching_face(image: np.ndarray, bbox: Dict) -> Optional[dlib.rectangle]:
    """Find the dlib face detection that best matches the provided bounding box"""
    dets, scores, idx = detector.run(image, 1, -1)

    best_match = None
    highest_iou = 0

    if len(dets):
        for d in dets:
            dlib_bbox = {
                "x_min": d.left(),
                "y_min": d.top(),
                "x_max": d.right(),
                "y_max": d.bottom()
            }
            iou_score = calculate_iou(bbox, dlib_bbox)

            if iou_score > highest_iou:
                highest_iou = iou_score
                best_match = d

    return best_match


def calc_normalized_matrix(landmarks: dlib.full_object_detection) -> np.ndarray:
    """
    Calculate normalized affine transformation matrix for facial landmarks
    Extracts eye, nose, and mouth positions to compute rotation angle
    """
    # Extract landmark points
    left_eye_points = [landmarks.part(i) for i in range(36, 42)]
    right_eye_points = [landmarks.part(i) for i in range(42, 48)]
    nose_points = [landmarks.part(i) for i in range(27, 36)]
    mouth_points = [landmarks.part(i) for i in range(48, 68)]

    # Calculate mean positions
    mean_left_eye = np.mean(np.array([(p.x, p.y) for p in left_eye_points]), axis=0)
    mean_right_eye = np.mean(np.array([(p.x, p.y) for p in right_eye_points]), axis=0)
    mean_nose = np.mean(np.array([(p.x, p.y) for p in nose_points]), axis=0)
    mean_mouth = np.mean(np.array([(p.x, p.y) for p in mouth_points]), axis=0)

    # Calculate rotation angle
    angle = np.degrees(
        np.arctan2(
            mean_right_eye[1] - mean_left_eye[1],
            mean_right_eye[0] - mean_left_eye[0],
        )
    )

    # Calculate center point
    center_x = (mean_left_eye[0] + mean_right_eye[0] + mean_nose[0]) / 3
    center_y = (mean_left_eye[1] + mean_right_eye[1] + mean_nose[1]) / 3

    return cv2.getRotationMatrix2D((center_x, center_y), angle, 1.0)


def crop_face_aligned(image: np.ndarray, bbox: Dict) -> Optional[Dict]:
    """
    Crop and align a face using dlib landmarks
    Returns both loose (with padding) and tight (exact) crops
    """
    gray_image = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY)

    try:
        best_face = find_best_matching_face(gray_image, bbox)
        if best_face is None:
            return None

        landmarks = predictor(gray_image, best_face)
        matrix = calc_normalized_matrix(landmarks)

        # Apply transformation
        transformed_image = cv2.warpAffine(
            image,
            matrix,
            (image.shape[1], image.shape[0]),
            flags=cv2.INTER_LINEAR,
        )

        # Calculate rotated landmarks
        landmarks_array = np.array([[p.x, p.y] for p in landmarks.parts()], dtype=np.float32)
        rotated_landmarks = cv2.transform(landmarks_array.reshape(1, -1, 2), matrix).reshape(-1, 2)

        # Calculate bounding rectangle
        x, y, w, h = cv2.boundingRect(rotated_landmarks.astype(np.int32))

        # Apply padding
        percentage = 0.2
        relaxation = max(MIN_PADDING, min(w * percentage, h * percentage))

        px = round(max(0, x - relaxation))
        py = round(max(0, y - relaxation))
        pw = round(min(w + (2 * relaxation), transformed_image.shape[1] - x))
        ph = round(min(h + (2 * relaxation), transformed_image.shape[0] - y))

        return {
            "loose": transformed_image[py:py + ph, px:px + pw],
            "tight": transformed_image[y:y + h, x:x + w],
        }
    except Exception as ex:
        app.logger.error(f"Face alignment failed: {ex}")
        app.logger.debug(traceback.format_exc())
        return None


def dlib_confidence_score(image: np.ndarray) -> Optional[Dict]:
    """
    Calculate dlib confidence score and pose type for a face image
    Returns score, pose type, and raw type index
    """
    try:
        rgb_image = cv2.cvtColor(image, cv2.COLOR_BGR2RGB)
        dets, scores, idx = detector.run(rgb_image, 1, -1)

        if len(dets):
            return {
                "score": float(scores[0]),
                "type": DLIB_SUBD[int(idx[0])],
                "type_raw": int(idx[0]),
            }
    except Exception as ex:
        app.logger.error(f"Confidence calculation failed: {ex}")
        app.logger.debug(traceback.format_exc())

    return None


def crop_face_simple(image: np.ndarray, bbox: Dict, percentage: float = 0.1) -> Optional[np.ndarray]:
    """
    Simple crop with relaxation percentage
    Fallback when dlib alignment fails
    """
    x_min, y_min = bbox["x_min"], bbox["y_min"]
    x_max, y_max = bbox["x_max"], bbox["y_max"]

    width = x_max - x_min
    height = y_max - y_min

    relaxation = min(width * percentage, height * percentage)

    x_min_relaxed = max(0, int(x_min - relaxation))
    y_min_relaxed = max(0, int(y_min - relaxation))
    x_max_relaxed = min(image.shape[1], int(x_max + relaxation))
    y_max_relaxed = min(image.shape[0], int(y_max + relaxation))

    return image[y_min_relaxed:y_max_relaxed, x_min_relaxed:x_max_relaxed]


def load_image_from_bytes(image_bytes: bytes) -> Optional[np.ndarray]:
    """Load image from bytes and convert to OpenCV format"""
    try:
        pil_image = Image.open(BytesIO(image_bytes))

        # Convert to RGB if needed
        if pil_image.mode != 'RGB':
            pil_image = pil_image.convert('RGB')

        # Convert PIL to OpenCV
        return cv2.cvtColor(np.array(pil_image), cv2.COLOR_RGB2BGR)
    except Exception as ex:
        app.logger.error(f"Failed to load image: {ex}")
        return None


def image_to_bytes(image: np.ndarray, format: str = 'JPEG') -> Optional[bytes]:
    """Convert OpenCV image to bytes"""
    try:
        # Convert BGR to RGB for PIL
        rgb_image = cv2.cvtColor(image, cv2.COLOR_BGR2RGB)
        pil_image = Image.fromarray(rgb_image)

        buffer = BytesIO()
        pil_image.save(buffer, format=format)
        return buffer.getvalue()
    except Exception as ex:
        app.logger.error(f"Failed to convert image to bytes: {ex}")
        return None


# ============================================================================
# API ENDPOINTS
# ============================================================================

@app.route('/health', methods=['GET'])
def health():
    """Health check endpoint"""
    return jsonify({
        "status": "healthy",
        "service": "quality-service",
        "version": "1.0.0"
    })


@app.route('/quality/assess', methods=['POST'])
def assess_quality():
    """
    Assess face quality for detected faces

    Request JSON:
    {
        "image": "base64_encoded_image",  # or use multipart file upload
        "faces": [
            {
                "box": {"x_min": 100, "y_min": 100, "x_max": 200, "y_max": 200}
            }
        ]
    }

    Response JSON:
    {
        "faces": [
            {
                "box": {...},
                "confidence": {
                    "score": 1.23,
                    "type": "front",
                    "type_raw": 0
                },
                "cropped_size": [width, height]
            }
        ]
    }
    """
    try:
        # Handle both JSON and multipart form data
        if request.is_json:
            data = request.get_json()
            if 'image' not in data:
                return jsonify({"error": "Missing 'image' in request"}), 400

            # Decode base64 image
            import base64
            image_bytes = base64.b64decode(data['image'])
            faces_data = data.get('faces', [])
        else:
            if 'file' not in request.files:
                return jsonify({"error": "Missing 'file' in request"}), 400

            file = request.files['file']
            image_bytes = file.read()

            # Parse faces from form data
            faces_json = request.form.get('faces', '[]')
            faces_data = json.loads(faces_json)

        # Load image
        image = load_image_from_bytes(image_bytes)
        if image is None:
            return jsonify({"error": "Failed to load image"}), 400

        # Process each face
        result_faces = []
        for face_data in faces_data:
            bbox = face_data.get('box')
            if not bbox:
                continue

            # Try dlib alignment first
            cropped_data = crop_face_aligned(image, bbox)

            if cropped_data and 'loose' in cropped_data:
                cropped_image = cropped_data['loose']
            else:
                # Fallback to simple crop
                cropped_image = crop_face_simple(image, bbox)

            if cropped_image is None:
                continue

            # Calculate quality metrics
            confidence = dlib_confidence_score(cropped_image)

            result_face = {
                "box": bbox,
                "confidence": confidence,
                "cropped_size": [cropped_image.shape[1], cropped_image.shape[0]]
            }

            # Include original face data
            for key in face_data:
                if key not in result_face:
                    result_face[key] = face_data[key]

            result_faces.append(result_face)

        return jsonify({"faces": result_faces})

    except Exception as ex:
        app.logger.error(f"Assessment failed: {ex}")
        app.logger.debug(traceback.format_exc())
        return jsonify({"error": str(ex)}), 500


@app.route('/quality/preprocess', methods=['POST'])
def preprocess_image():
    """
    Preprocess and enhance image quality

    Request: multipart/form-data with 'file'
    Response: Enhanced image file
    """
    try:
        if 'file' not in request.files:
            return jsonify({"error": "Missing 'file' in request"}), 400

        file = request.files['file']
        image_bytes = file.read()

        # Load image
        image = load_image_from_bytes(image_bytes)
        if image is None:
            return jsonify({"error": "Failed to load image"}), 400

        # Apply histogram equalization for better face detection
        gray = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY)
        equalized = cv2.equalizeHist(gray)
        enhanced = cv2.cvtColor(equalized, cv2.COLOR_GRAY2BGR)

        # Convert back to bytes
        result_bytes = image_to_bytes(enhanced)
        if result_bytes is None:
            return jsonify({"error": "Failed to encode result"}), 500

        from flask import send_file
        return send_file(
            BytesIO(result_bytes),
            mimetype='image/jpeg',
            as_attachment=False,
            download_name='enhanced.jpg'
        )

    except Exception as ex:
        app.logger.error(f"Preprocessing failed: {ex}")
        app.logger.debug(traceback.format_exc())
        return jsonify({"error": str(ex)}), 500


@app.route('/quality/detect', methods=['POST'])
def detect_enhanced():
    """
    Enhanced face detection with quality metrics
    Uses dlib detector with multiple orientations

    Request: multipart/form-data with 'file'
    Response JSON:
    {
        "faces": [
            {
                "box": {"x_min": ..., "y_min": ..., "x_max": ..., "y_max": ...},
                "confidence": {"score": ..., "type": ...},
                "cropped_size": [width, height]
            }
        ]
    }
    """
    try:
        if 'file' not in request.files:
            return jsonify({"error": "Missing 'file' in request"}), 400

        file = request.files['file']
        image_bytes = file.read()

        # Load image
        image = load_image_from_bytes(image_bytes)
        if image is None:
            return jsonify({"error": "Failed to load image"}), 400

        # Convert to grayscale for detection
        gray = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY)

        # Run dlib detector with multiple orientations
        dets, scores, idx = detector.run(gray, 1, -1)

        result_faces = []
        for i, d in enumerate(dets):
            bbox = {
                "x_min": d.left(),
                "y_min": d.top(),
                "x_max": d.right(),
                "y_max": d.bottom()
            }

            # Crop face
            cropped = crop_face_simple(image, bbox)
            if cropped is None:
                continue

            confidence = {
                "score": float(scores[i]),
                "type": DLIB_SUBD[int(idx[i])],
                "type_raw": int(idx[i])
            }

            result_faces.append({
                "box": bbox,
                "confidence": confidence,
                "cropped_size": [cropped.shape[1], cropped.shape[0]]
            })

        return jsonify({"faces": result_faces})

    except Exception as ex:
        app.logger.error(f"Detection failed: {ex}")
        app.logger.debug(traceback.format_exc())
        return jsonify({"error": str(ex)}), 500


if __name__ == '__main__':
    port = int(os.environ.get('QUALITY_SERVICE_PORT', 6001))
    print(f"Starting Stash Face Quality Service on port {port}...")
    app.run(host='0.0.0.0', port=port, debug=False)
