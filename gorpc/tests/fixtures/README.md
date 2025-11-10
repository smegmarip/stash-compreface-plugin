# Test Fixtures

This directory contains test data for the Stash Compreface plugin test suite.

## Images

The `images/` directory should contain test images for face detection and recognition tests.

### Required Test Images

For integration tests to run, you need to provide:

- `test_face.jpg` - A clear image with at least one visible face

### Recommended Test Images

- `single_face.jpg` - Image with exactly one face
- `multiple_faces.jpg` - Image with 2-5 faces
- `no_face.jpg` - Image with no faces
- `low_quality.jpg` - Blurry or small face image
- `profile.jpg` - Profile/side view of face

## Mock Responses

The `responses/` directory contains mock JSON responses from external services for unit testing.

### Compreface Responses

- `compreface/detection_response.json` - Face detection API response
- `compreface/recognition_response.json` - Face recognition API response
- `compreface/subject_list.json` - List subjects response

### Stash Responses

- `stash/image_query.json` - GraphQL image query response
- `stash/performer_query.json` - GraphQL performer query response

## Note on Test Data

Test images are **not** committed to the repository due to:
- File size concerns
- Privacy considerations
- Copyright issues

To run integration tests:

1. Create the `tests/fixtures/images/` directory
2. Add your own test images (JPEG format recommended)
3. Ensure images have appropriate permissions
4. Images should be clear, well-lit faces for best results

You can use:
- Public domain face images
- Generated/synthetic faces
- Your own photos (with appropriate consent)
