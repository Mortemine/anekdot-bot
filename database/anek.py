from database.base import Base
from sqlalchemy.orm import Mapped
from sqlalchemy.orm import mapped_column
from sqlalchemy import ForeignKey


class Anek(Base):
    __tablename__ = 'anekdots'

    id: Mapped[int] = mapped_column(primary_key=True, nullable=False, autoincrement=True)
    category: Mapped[int] = mapped_column(ForeignKey("categories.id"), nullable=False)
    text: Mapped[str] = mapped_column(nullable=False)

    def __repr__(self) -> str:
        return f'Anek(id={self.id!r}, category={self.category!r}, text={self.text!r})'
