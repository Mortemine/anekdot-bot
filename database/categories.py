from base import Base
from sqlalchemy.orm import Mapped
from sqlalchemy.orm import mapped_column


class Categories(Base):
    __tablename__ = 'categories'

    id: Mapped[int] = mapped_column(primary_key=True, nullable=False)
    name: Mapped[str] = mapped_column(nullable=False)

    def __repr__(self) -> str:
        return f'Categories(id={self.id!r}, name={self.name!r})'
    