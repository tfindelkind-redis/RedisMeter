"""
Exception classes for the RedisMeter SDK.
"""


class RedisMeterError(Exception):
    """Base exception for RedisMeter SDK errors."""
    
    def __init__(self, message: str, code: str = None, details: dict = None):
        super().__init__(message)
        self.message = message
        self.code = code
        self.details = details or {}


class APIError(RedisMeterError):
    """Error returned from the RedisMeter API."""
    
    def __init__(self, message: str, status_code: int, code: str = None, details: dict = None):
        super().__init__(message, code, details)
        self.status_code = status_code


class AuthenticationError(RedisMeterError):
    """Authentication failed."""
    pass


class NotFoundError(RedisMeterError):
    """Resource not found."""
    pass


class ValidationError(RedisMeterError):
    """Request validation failed."""
    
    def __init__(self, message: str, field: str = None, details: dict = None):
        super().__init__(message, "VALIDATION_ERROR", details)
        self.field = field


class RateLimitError(RedisMeterError):
    """Rate limit exceeded."""
    
    def __init__(self, message: str, retry_after: int = None):
        super().__init__(message, "RATE_LIMIT_EXCEEDED")
        self.retry_after = retry_after


class TimeoutError(RedisMeterError):
    """Request timed out."""
    pass


class ConnectionError(RedisMeterError):
    """Failed to connect to the RedisMeter API."""
    pass
