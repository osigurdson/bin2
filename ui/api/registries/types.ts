export interface Registry {
  id: string;
  name: string;
  sizeBytes: number;
}

export interface ListRegistriesResponse {
  registries: Registry[];
}
