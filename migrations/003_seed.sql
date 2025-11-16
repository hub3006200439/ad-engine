INSERT INTO campaigns
(id, name, pricing_model, bid_micros, budget_total, budget_daily, active, frequency_cap, targeting_json)
VALUES
(uuid_generate_v4(), 'CPM EN Video',     'CPM', 2000000, 500000000, 100000000, TRUE,  5,
 '{"languages":["en"],"countries":["US","GB"],"categories":["tech"],"placements":["preroll"]}'),
(uuid_generate_v4(), 'CPC RU Gaming',    'CPC', 1500000, 300000000,  80000000, TRUE, 10,
 '{"languages":["ru"],"countries":["RU"],"categories":["gaming"],"placements":["preroll","midroll"]}'),
(uuid_generate_v4(), 'CPM RU Finance',   'CPM', 2500000, 400000000, 120000000, TRUE,  3,
 '{"languages":["ru"],"countries":["RU"],"categories":["finance"],"placements":["preroll"]}'),
(uuid_generate_v4(), 'CPC EN Mobile',    'CPC', 1800000, 350000000,  90000000, TRUE,  8,
 '{"languages":["en"],"countries":["US","CA"],"categories":["apps"],"placements":["midroll"]}'),
(uuid_generate_v4(), 'CPM Global Brand', 'CPM', 1000000, 800000000, 200000000, TRUE, 20,
 '{"languages":["en","ru"],"countries":["US","RU","EU"],"categories":["brand"],"placements":["preroll","midroll","postroll"]}');

INSERT INTO creatives (campaign_id, duration_sec, video_url, landing_url, lang, placement, category)
SELECT
    c.id,
    (10 + (random() * 20)::int),
    'https://cdn.example.com/video_' || c.id || '_' || g || '.mp4',
    'https://advertiser.example.com/landing/' || c.id || '/' || g,
    (c.targeting_json->'languages'->>0),
    (c.targeting_json->'placements'->>0),
    (c.targeting_json->'categories'->>0)
FROM campaigns c
CROSS JOIN generate_series(1, 10) AS g;

-- 10 000 показов + клики (≈ 3 000) => > 10k событий
DO $$
DECLARE
    i      INT;
    imp_id UUID;
BEGIN
    FOR i IN 1..10000 LOOP
        INSERT INTO impressions (token, campaign_id, creative_id, user_id, session_id, created_at)
        SELECT
            md5(random()::text || clock_timestamp()::text),
            c.id,
            cr.id,
            uuid_generate_v4(),
            uuid_generate_v4(),
            NOW() - (random() * INTERVAL '7 days')
        FROM campaigns c
        JOIN creatives cr ON cr.campaign_id = c.id
        ORDER BY random()
        LIMIT 1
        RETURNING id INTO imp_id;

        IF random() < 0.3 THEN
            INSERT INTO clicks (impression_id, token, created_at)
            VALUES (imp_id, md5(random()::text || clock_timestamp()::text), NOW())
            ON CONFLICT (impression_id) DO NOTHING;
        END IF;
    END LOOP;
END $$;
