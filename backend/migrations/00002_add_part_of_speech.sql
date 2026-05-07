-- +goose Up
ALTER TABLE vocab_items
    ADD COLUMN part_of_speech TEXT NOT NULL DEFAULT '';

ALTER TABLE vocab_items
    ADD CONSTRAINT vocab_items_part_of_speech_check CHECK (
        part_of_speech IN (
            '',
            'noun',
            'verb',
            'adjective',
            'adverb',
            'phrase',
            'idiom',
            'phrasal_verb',
            'preposition',
            'conjunction',
            'interjection',
            'determiner',
            'pronoun',
            'other'
        )
    );

-- +goose Down
ALTER TABLE vocab_items
    DROP CONSTRAINT vocab_items_part_of_speech_check;

ALTER TABLE vocab_items
    DROP COLUMN part_of_speech;
