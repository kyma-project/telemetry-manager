# Test Results: Self-Monitor

## Overview

The main KPIs to track performance changes are **scrape samples per sec** and **total series created**. These values
should be in the range of 15-22 samples/sec and 200-350 series, respectively.
Other metrics to track are **CPU** and **memory usage** of the self-monitor Pods. Both are directly influenced by the
number of series created and the scrape samples/sec: more samples and series created increase the memory and CPU usage
of the self-monitor Pods.

## Results

| Version/Test | Default (ci-self-monitor) | | | | | |
|--|--|--|--|--|--|--|
| Version | Scrape Samples/sec | Total Series Created | WAL Storage Size/bytes | Head Chunk Storage Size in bytes | Pod Memory Usage(MB) | Pod CPU Usage |
| 2.45.5 | 15.4 | 157 | - | 131072 | 62 | 0 |
| 2.45.5(new) | 15.4 | 325 | 127633 | 0 | 43 | 0 |
| 2.53.0 | 20.4 | 210 | - | 0 | 36 | 0 |
| 2.53.1 | 20 | 333 | 135557 | 0 | 37 | 0 |
| 2.53.2 | 20 | 367 | 138347 | 0 | 40 | 0 |
| 2.53.3 | 20 | 307 | 127673 | 0 | 35 | 0 |
| 3.0.1 | 21 | 333 | 138210 | 0 | 43 | 0 |
| 3.1.0 | 21 | 336 | 133158 | 0 | 39 | 0 |
| 3.2.0 | 21 | 332 | 131506 | 0 | 38 | 0 |
| 3.4.0 | 21 | 295 | 114617 | 0 | 49 | 0 |
| 3.4.1 | 21 | 337 | 124497 | 0 | 42 | 0 |
| 3.5.0 | 21 | 306 | 119578 | 0 | 36 | 0 |
| 3.6.0 | 21 | 281 | 111475 | 0 | 35 | 0 |
| 3.7.1 | 21 | 257 | 104356 | 0 | 35 | 0 |
| 3.7.3 | 21 | 273 | 111354 | 0 | 40 | 0 |
| 3.8.0 | 21 | 247 | 107263 | 0 | 36 | 0 |
| 3.9.1 | 24 | 235 | 103072 | 0 | 36 | 0 |
