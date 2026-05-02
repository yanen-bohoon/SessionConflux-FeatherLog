package feishu

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "mime/multipart"
    "net/http"
)

// CreateFolder creates a folder in Feishu Drive root and returns its token.
func CreateFolder(token, name string) (string, error) {
    body := fmt.Sprintf(`{"name":"%s","folder_token":""}`, name)
    req, err := http.NewRequest("POST", BaseURL+"/drive/v1/files/create_folder",
        bytes.NewReader([]byte(body)))
    if err != nil {
        return "", err
    }
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", fmt.Errorf("create folder failed: %w", err)
    }
    defer resp.Body.Close()

    var result struct {
        Code int    `json:"code"`
        Msg  string `json:"msg"`
        Data struct {
            Token string `json:"token"`
        } `json:"data"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", fmt.Errorf("create folder decode: %w", err)
    }
    if result.Code != 0 {
        return "", fmt.Errorf("create folder API error: code=%d msg=%s", result.Code, result.Msg)
    }
    return result.Data.Token, nil
}

// UploadFile uploads a file to a folder. Returns the file_token.
// Uses upload_all API (≤20MB limit).
func UploadFile(token, folderToken, fileName string, data []byte) (string, error) {
    var buf bytes.Buffer
    w := multipart.NewWriter(&buf)

    _ = w.WriteField("file_name", fileName)
    _ = w.WriteField("parent_type", "explorer")
    _ = w.WriteField("parent_node", folderToken)
    _ = w.WriteField("size", fmt.Sprintf("%d", len(data)))

    part, err := w.CreateFormFile("file", fileName)
    if err != nil {
        return "", err
    }
    if _, err := part.Write(data); err != nil {
        return "", err
    }
    w.Close()

    req, err := http.NewRequest("POST", BaseURL+"/drive/v1/files/upload_all", &buf)
    if err != nil {
        return "", err
    }
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Content-Type", w.FormDataContentType())

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", fmt.Errorf("upload failed: %w", err)
    }
    defer resp.Body.Close()

    var result struct {
        Code int    `json:"code"`
        Msg  string `json:"msg"`
        Data struct {
            Token string `json:"file_token"`
        } `json:"data"`
    }
    respBytes, _ := io.ReadAll(resp.Body)
    if err := json.Unmarshal(respBytes, &result); err != nil {
        return "", fmt.Errorf("upload decode: %w, body=%s", err, string(respBytes))
    }
    if result.Code != 0 {
        return "", fmt.Errorf("upload API error: code=%d msg=%s", result.Code, result.Msg)
    }
    return result.Data.Token, nil
}

// DownloadFile downloads a file by its token.
func DownloadFile(token, fileToken string) ([]byte, error) {
    req, err := http.NewRequest("GET",
        BaseURL+"/drive/v1/files/"+fileToken+"/download", nil)
    if err != nil {
        return nil, err
    }
    req.Header.Set("Authorization", "Bearer "+token)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("download failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("download HTTP %d: %s", resp.StatusCode, string(body))
    }

    return io.ReadAll(resp.Body)
}

// FindOrCreateFolder finds a folder by name in root, or creates it.
func FindOrCreateFolder(token, name string) (string, error) {
    // Try to list root files and find by name
    req, err := http.NewRequest("GET",
        BaseURL+"/drive/v1/files?page_size=100&direction=ASC", nil)
    if err != nil {
        return "", err
    }
    req.Header.Set("Authorization", "Bearer "+token)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    var result struct {
        Code int `json:"code"`
        Msg  string `json:"msg"`
        Data struct {
            Files []struct {
                Token string `json:"token"`
                Name  string `json:"name"`
                Type  string `json:"type"`
            } `json:"files"`
        } `json:"data"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", fmt.Errorf("list files decode: %w", err)
    }
    if result.Code != 0 {
        return "", fmt.Errorf("list files API error: code=%d msg=%s", result.Code, result.Msg)
    }

    for _, f := range result.Data.Files {
        if f.Name == name && f.Type == "folder" {
            return f.Token, nil
        }
    }
    // Not found, create it
    return CreateFolder(token, name)
}

// FileInfo holds basic file metadata from the Drive API.
type FileInfo struct {
	Token string
	Name  string
	Type  string
}

// DeleteFile deletes a file by its token. Returns nil even if the file is already gone.
func DeleteFile(token, fileToken string) error {
	req, err := http.NewRequest("DELETE",
		BaseURL+"/drive/v1/files/"+fileToken, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil // already deleted
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete file HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// ListFiles lists files in the root or a specific folder. Paginates through all pages.
func ListFiles(token, folderToken string) ([]FileInfo, error) {
	var allFiles []FileInfo
	pageToken := ""

	for {
		url := BaseURL + "/drive/v1/files?page_size=100"
		if folderToken != "" {
			url += "&folder_token=" + folderToken
		}
		if pageToken != "" {
			url += "&page_token=" + pageToken
		}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("list files: %w", err)
		}

		var result struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				Files     []struct {
					Token string `json:"token"`
					Name  string `json:"name"`
					Type  string `json:"type"`
				} `json:"files"`
				HasMore   bool   `json:"has_more"`
				PageToken string `json:"page_token"`
			} `json:"data"`
		}
		decodeErr := json.NewDecoder(resp.Body).Decode(&result)
			resp.Body.Close()
			if decodeErr != nil {
				return nil, fmt.Errorf("list files decode: %w", decodeErr)
			}
			if result.Code != 0 {
				return nil, fmt.Errorf("list files API: code=%d msg=%s", result.Code, result.Msg)
			}
			for _, f := range result.Data.Files {
				allFiles = append(allFiles, FileInfo{
					Token: f.Token,
					Name:  f.Name,
					Type:  f.Type,
				})
			}
			if !result.Data.HasMore {
				break
			}
			pageToken = result.Data.PageToken
	}
	return allFiles, nil
}
