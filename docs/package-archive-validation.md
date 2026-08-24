# Package archive validation

This records an end-to-end local validation of the Send archive policy on 20 August 2026.

## Environment and scope

- The real Filebox application ran locally with S3 disabled and local target storage enabled.
- Packages were created through the actual `POST /api/packages` endpoint.
- Archives were built by the actual background package-preparation worker.
- Source files contained real bytes, and generated archives were fetched through the real HTTP download endpoint.
- TUS-over-HTTP transfer and OAuth login were bypassed with direct database and filesystem seeding to keep the large scenarios practical. Packaging, archive creation, delivery, and integrity verification were not bypassed.

## Results

| # | Setup | Result |
|---:|---|---|
| 1 | 3 small files | 3 direct artifacts |
| 2 | Exactly 10 small files | 10 direct artifacts; the boundary was honored |
| 3 | 11 small files | 1 ZIP, downloaded and CRC-verified |
| 4 | 11 files, some sharing the same original name | Names were deduplicated to `report.txt`, `report (2).txt`, `report (3).txt`, and so on |
| 5 | 1 × 12 GiB plus 10 small files; total approximately 12.9 GiB | All 11 files were placed in one ZIP, including the 12 GiB file |
| 6 | 1 × 15 GiB plus 10 × 9 GiB; total 105 GiB | The 15 GiB file stayed direct; the ten 9 GiB files became one 90 GiB ZIP |
| 7 | 15 × 8 GiB; total 120 GiB | Split into two ZIP parts of 96 GiB and 24 GiB, named `part-001-of-002` and `part-002-of-002` |
| 8 | Filename containing a colon | The colon was replaced with `_` inside the ZIP entry name |

Every result matched the planner in `internal/api/package_archive_plan.go`. All generated ZIPs—including the 90 GiB archive and the split 96/24 GiB archives—passed `unzip` structural and integrity checks after real HTTP downloads.

## Important branch precedence

The per-file 10 GiB split applies only when the package's combined source size exceeds 100 GiB:

1. When there are more than 10 files and their combined size is at most 100 GiB, all files are ZIP members regardless of individual size.
2. Only when the combined size exceeds 100 GiB do files of 10 GiB or more remain direct while smaller files are grouped into ZIP parts.

Consequently, a 50 GiB source in an otherwise sub-100 GiB package is placed inside the package ZIP.

## Automated coverage

The local validation complements the automated planner, worker, API, migration, and artifact-delivery tests in `internal/api` and `internal/db`. The automated tests protect boundary and failure behavior; this report confirms the same policy using the running application and large real archive output.
