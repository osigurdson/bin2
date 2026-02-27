export interface Repository {
  id: string;
  name: string;
  lastPush: string;
  lastTag: string | null;
}

export interface ListRepositoriesResponse {
  repositories: Repository[];
}
