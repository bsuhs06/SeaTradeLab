---
layout: default
title: Datasets
description: Downloadable datasets from the SeaTradeLab project.
permalink: /datasets/
---

# Datasets

Periodically released datasets from the SeaTradeLab project. All data is derived from publicly available AIS sources.

<!-- Add dataset cards below as you publish them. Example format:

<div class="dataset-card">
  <div class="dataset-info">
    <h3>Baltic STS Events — Q1 2026</h3>
    <p>Ship-to-ship transfer detections in the Baltic Sea from January through March 2026. Includes vessel pairs, timestamps, coordinates, and proximity details.</p>
    <div class="dataset-meta">
      <span>CSV</span>
      <span>2.4 MB</span>
      <span>1,247 records</span>
    </div>
  </div>
  <a href="{{ '/assets/datasets/baltic-sts-q1-2026.csv' | relative_url }}" class="btn-download">⬇ Download</a>
</div>

<div class="dataset-card">
  <div class="dataset-info">
    <h3>Russian Port Visits — 2025</h3>
    <p>Non-Russian flagged vessels detected visiting Russian oil terminals (Primorsk, Ust-Luga, Vysotsk) during 2025.</p>
    <div class="dataset-meta">
      <span>CSV</span>
      <span>890 KB</span>
      <span>3,412 records</span>
    </div>
  </div>
  <a href="{{ '/assets/datasets/russian-port-visits-2025.csv' | relative_url }}" class="btn-download">⬇ Download</a>
</div>

-->

*No datasets published yet. Check back soon — the first release will include Baltic STS detections and AIS gap events.*

## How to use

Datasets are provided as CSV files. Each download page includes a data dictionary describing the columns. You can load them with Python, R, Excel, or any tool that handles CSV.

```python
import pandas as pd
df = pd.read_csv("baltic-sts-q1-2026.csv")
```

## License

All datasets are released under the [MIT License](https://github.com/bsuhs/seatradelab/blob/main/LICENSE), same as the project source code. The underlying AIS data comes from publicly available sources (primarily Finnish Digitraffic).
