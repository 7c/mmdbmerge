# mmdbmerge
A tool to merge multiple MaxMind DB files into a single file. It will skip all data from all files and add "from" property to indicate which file the data came from. You may want to use [mmdbinspect](https://github.com/maxmind/mmdbinspect) or [mmdimport](https://github.com/7c/mmdbimport) to view the data.

## Installation
Easy way to install this tool is to use `go install` command.
```bash
go clean -cache
go install github.com/7c/mmdbmerge@v0.0.1
```

## Build
If you want to build this tool from source, you can use `make` command.
```bash
$ make build
$ bin/mmdbmerge -h
```

## Usage
```bash
$ mmdbmerge input1.mmdb input2.mmdb [input3.mmdb ...] -o combined.mmdb [--debug]
2025/01/29 18:31:18 VALID: Valid file: demo/a.mmdb
2025/01/29 18:31:18 VALID: Valid file: demo/b.mmdb
2025/01/29 18:31:18 Stats: demo/a.mmdb networks: 2166 (IPs: 681413, skipped: 0)
2025/01/29 18:31:18 Stats: demo/b.mmdb networks: 2166 (IPs: 681413, skipped: 0)
2025/01/29 18:31:18 Final Stats: Total networks: 4332 (IPs: 1362826, skipped: 0)
2025/01/29 18:31:18 Output: combined.mmdb contains 906 networks (IPs: 681413)
% mmdbimport -v combined.mmdb 
MMDB file: combined.mmdb
  Build Timestamp: 2025-01-29T18:31:18-05:00

Database Information:
  Binary Format: 2.0
  IP Version: 6
  Record Size: 28 bits
  Node Count: 7436

Metadata:
  Database Type: Combined-DB
  Description:
    en: Combined a, b
  Languages: en

Statistics:
  Total Networks: 906

$ mmdbinspect -db combined.mmdb 212.102.44.4
[
    {
        "Database": "combined.mmdb",
        "Records": [
            {
                "Network": "212.102.32.0/19",
                "Record": {
                    "from": "b"
                }
            }
        ],
        "Lookup": "212.102.44.4"
    }
]
```

the mmdb driver optimizes overlaps internally, thanks!
