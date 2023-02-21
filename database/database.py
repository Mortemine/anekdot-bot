from sqlalchemy import create_engine
from sqlalchemy.orm import DeclarativeBase
from sqlalchemy.orm import Mapped
from sqlalchemy.orm import mapped_column
from sqlalchemy import ForeignKey
from sqlalchemy.orm import Session
from sqlalchemy import select


class Base(DeclarativeBase):
    pass


class Anek(Base):
    __tablename__ = 'anekdots'

    id: Mapped[int] = mapped_column(primary_key=True, nullable=False, autoincrement=True)
    category: Mapped[int] = mapped_column(ForeignKey("categories.id"), nullable=False)
    text: Mapped[str] = mapped_column(nullable=False)

    def __repr__(self) -> str:
        return f'Anek(id={self.id!r}, category={self.category!r}, text={self.text!r})'


class Categories(Base):
    __tablename__ = 'categories'

    id: Mapped[int] = mapped_column(primary_key=True, nullable=False)
    name: Mapped[str] = mapped_column(nullable=False)

    def __repr__(self) -> str:
        return f'Categories(id={self.id!r}, name={self.name!r})'


engine = create_engine("sqlite://", echo=True)
Base.metadata.create_all(engine)

with Session(engine) as session:
    with open('cleaner/categories.txt', 'r', encoding='UTF-8') as f:
        categories = f.read().split('\n')
        for line in categories:
            category_id = line[0]
            cat_start = line.find(',') + 2
            cat_text = line[cat_start:]
            category = Categories(
                id=category_id,
                name=cat_text
            )
            session.add(category)
    with open('cleaner/no_insert_aneks.txt', 'r', encoding='UTF-8') as f:
        aneks = f.read().split('\n')
        for line in aneks:
            anek_id = line[0]
            text_start = line.find(',') + 2
            anek_text = line[text_start:]
            anek = Anek(
                id=anek_id,
                text=anek_text
            )
            session.add(anek)
    session.commit()

