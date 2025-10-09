# Yeet-Tube: A Personal YouTube Archival Tool

Inspired by the retro-futuristic aesthetic of the Time Variance Authority (TVA) from the Loki series, Yeet-Tube is a terminal-based tool for archiving your favorite YouTube videos. It provides a unique, immersive experience for managing your personal video collection.

**For Personal Use Only:** This tool is intended for creating personal backups of your own content or publicly available videos for offline viewing. Please respect copyright laws and the terms of service of YouTube.

![Screenshot](ms_minutes.png)

## Features

*   **TVA-Inspired Interface:** A retro-style terminal UI built with Bubble Tea.
*   **YouTube Video Downloader:** Download videos from YouTube using `yt-dlp`.
*   **Download Progress:** Monitor download progress in real-time.
*   **Archive History:** Keep a record of your downloaded videos.
*   **Metadata Viewer:** View detailed information about your archived videos.

## Tech Stack

*   **Go:** The core application is written in Go.
*   **Bubble Tea:** A Go library for building terminal user interfaces.
*   **yt-dlp:** A command-line program to download videos from YouTube and other video sites.

## Prerequisites

Before you begin, ensure you have the following installed:

*   **Go:** [https://golang.org/doc/install](https://golang.org/doc/install)
*   **yt-dlp:** [https://github.com/yt-dlp/yt-dlp#installation](https://github.com/yt-dlp/yt-dlp#installation)

## Installation

1.  Clone the repository:
    ```sh
    git clone https://github.com/your-username/yeet-tube.git
    ```
2.  Navigate to the project directory:
    ```sh
    cd yeet-tube
    ```
3.  Install the Go dependencies:
    ```sh
    go mod tidy
    ```

## Usage

1.  Run the application:
    ```sh
    go run main.go
    ```
2.  Paste a YouTube video URL into the input field and press Enter.
3.  The video will be downloaded to the project directory, and its metadata will be saved in `downloads.json`.
