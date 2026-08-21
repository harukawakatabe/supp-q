# Private Recognition Production Gate

Live provider connectivity is not accuracy acceptance. Production recognition
requires an explicitly authorized private set of 30–50 images. Keep images and
reports under ignored `testdata/recognition/private/` and `reports/private/`.
Do not send an image to any provider unless the owner has authorized that exact
destination and purpose.

## Dataset

Cover all three roles (`front`, `facts`, `expiry`), Chinese and English labels,
and glare, blur, crop, low contrast, and month/year-only expiry evidence. The
evaluation JSON contains no image bytes:

```json
{
  "name": "private-labels-2026-08",
  "samples": [{
    "id": "opaque-001-front",
    "role": "front",
    "expected": {"productName": "Example", "count": "60"},
    "predicted": {"productName": "Example", "count": "60"},
    "status": "recognized",
    "confidence": 0.91,
    "correctedFields": [],
    "latencyMs": 4200,
    "providerError": false
  }]
}
```

`expected` is human-annotated label truth. `predicted` is the persisted candidate
before human edits. `correctedFields` records fields actually changed on the
confirmation screen. Month/year evidence such as `03/2027` stays month/year;
never invent a day to make an exact-date field pass.

## Gate

From `uni/`:

```bash
make recognition-eval INPUT=testdata/recognition/private/evaluation.json \
  OUTPUT=reports/private/recognition-report.json
```

The fixed initial thresholds are:

- field accuracy at least 90%
- image correction rate at most 25%
- unrecognized rate at most 15%
- false-high-confidence rate at most 5%
- provider failure rate at most 5%
- p95 end-to-end latency at most 45 seconds

The command exits non-zero on a failed metric, missing role, malformed sample,
or a production set outside 30–50 images. Threshold changes require a recorded
product decision; do not lower them merely to make a provider pass.
