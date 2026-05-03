package feishu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

// CreateFolder creates a folder. If parentToken is given, creates under it; otherwise root.
func (c *Client) CreateFolder(name string, parentToken ...string) (string, error) {
	token, err := c.GetTenantToken()
	if err != nil {
		return "", err
	}

	parent := ""
	if len(parentToken) > 0 {
		parent = parentToken[0]
	}
	body := fmt.Sprintf(`{"name":"%s","folder_token":"%s"}`, name, parent)
	req, err := http.NewRequest("POST", c.baseURL+"/drive/v1/files/create_folder",
		bytes.NewReader([]byte(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
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
func (c *Client) UploadFile(folderToken, fileName string, data []byte) (string, error) {
	token, err := c.GetTenantToken()
	if err != nil {
		return "", err
	}

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

	req, err := http.NewRequest("POST", c.baseURL+"/drive/v1/files/upload_all", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.httpClient.Do(req)
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

// ProgressFunc is called during download with bytes received and total (0 if unknown).
type ProgressFunc func(downloaded, total int64)

// DownloadFile downloads a file by its token. If progress is non-nil, it is called
// periodically with bytes downloaded so far and total size.
func (c *Client) DownloadFile(fileToken string, progress ProgressFunc) ([]byte, error) {
	token, err := c.GetTenantToken()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET",
		c.baseURL+"/drive/v1/files/"+fileToken+"/download", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("download HTTP %d: %s", resp.StatusCode, string(body))
	}

	if progress == nil {
		return io.ReadAll(resp.Body)
	}

	total := resp.ContentLength
	progress(0, total)
	var buf bytes.Buffer
	chunk := make([]byte, 32*1024)
	var downloaded int64
	for {
		n, err := resp.Body.Read(chunk)
		if n > 0 {
			buf.Write(chunk[:n])
			downloaded += int64(n)
			progress(downloaded, total)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("download read: %w", err)
		}
	}
	return buf.Bytes(), nil
}

// FindOrCreateFolder finds a folder by name under parentToken (or root), or creates it.
func (c *Client) FindOrCreateFolder(name string, parentToken ...string) (string, error) {
	parent := ""
	if len(parentToken) > 0 {
		parent = parentToken[0]
	}
	files, err := c.ListFiles(parent)
	if err != nil {
		return "", err
	}
	for _, f := range files {
		if f.Name == name && f.Type == "folder" {
			return f.Token, nil
		}
	}
	return c.CreateFolder(name, parent)
}

// FileInfo holds basic file metadata from the Drive API.
type FileInfo struct {
	Token string
	Name  string
	Type  string
}

// DeleteFile deletes a file by its token. Returns nil even if the file is already gone.
func (c *Client) DeleteFile(fileToken string) error {
	token, err := c.GetTenantToken()
	if err != nil {
		return err
	}

	req, err := http.NewRequest("DELETE",
		c.baseURL+"/drive/v1/files/"+fileToken+"?type=file", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
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
func (c *Client) ListFiles(folderToken string) ([]FileInfo, error) {
	token, err := c.GetTenantToken()
	if err != nil {
		return nil, err
	}

	var allFiles []FileInfo
	pageToken := ""

	for {
		url := c.baseURL + "/drive/v1/files?page_size=100"
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

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("list files: %w", err)
		}

		var result struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				Files []struct {
					Token string `json:"token"`
					Name  string `json:"name"`
					Type  string `json:"type"`
				} `json:"files"`
				HasMore   bool   `json:"has_more"`
				PageToken string `json:"next_page_token"`
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
