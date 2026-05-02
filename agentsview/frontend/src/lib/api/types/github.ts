export interface PublishResponse {
  gist_id: string;
  gist_url: string;
  view_url: string;
  raw_url: string;
}

export interface GithubConfig {
  configured: boolean;
}

export interface SetGithubConfigResponse {
  success: boolean;
  username: string;
}
