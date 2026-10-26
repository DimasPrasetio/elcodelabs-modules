// The API envelope (.claude/rules/api-standard.md), as the frontend sees it.

export interface ApiSuccess<T> {
  success: true;
  message: string;
  data: T;
}

export interface PageMeta {
  page: number;
  limit: number;
  total: number;
  total_pages: number;
}

export interface ApiPage<T> extends ApiSuccess<T[]> {
  meta: PageMeta;
}

export interface ApiFieldError {
  field: string;
  message: string;
}

export interface ApiFailure {
  success: false;
  message: string;
  errors: ApiFieldError[] | unknown[];
}
